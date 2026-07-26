package cache

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type KeyInput struct {
	TenantID           string
	Prefix             string
	KeyFiles           map[string][]byte
	Commands           []string
	RunnerImageVersion string
	Env                map[string]string
}

func Key(input KeyInput) (string, string) {
	normalized := map[string]any{
		"tenant_id":            input.TenantID,
		"prefix":               input.Prefix,
		"commands":             input.Commands,
		"runner_image_version": input.RunnerImageVersion,
		"os":                   runtime.GOOS,
		"arch":                 runtime.GOARCH,
	}
	fileNames := make([]string, 0, len(input.KeyFiles))
	files := map[string]string{}
	for name := range input.KeyFiles {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	for _, name := range fileNames {
		sum := sha256.Sum256(input.KeyFiles[name])
		files[name] = hex.EncodeToString(sum[:])
	}
	normalized["key_files"] = files
	envNames := make([]string, 0, len(input.Env))
	env := map[string]string{}
	for name := range input.Env {
		envNames = append(envNames, name)
	}
	sort.Strings(envNames)
	for _, name := range envNames {
		env[name] = input.Env[name]
	}
	normalized["env"] = env
	payload, _ := json.Marshal(normalized)
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	return hash, fmt.Sprintf("cache/sha256/%s/%s.tar.zst", hash[:2], hash)
}

type Client struct {
	minio  *minio.Client
	bucket string
}

func NewClient(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*Client, error) {
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: useSSL})
	if err != nil {
		return nil, err
	}
	return &Client{minio: client, bucket: bucket}, nil
}

func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.minio.BucketExists(ctx, c.bucket)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return c.minio.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{})
}

func (c *Client) UploadArchive(ctx context.Context, objectKey string, root string, paths []string) (string, int64, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, p)
		}
		if err := addPath(tw, root, abs); err != nil {
			_ = tw.Close()
			_ = gz.Close()
			return "", 0, err
		}
	}
	if err := tw.Close(); err != nil {
		return "", 0, err
	}
	if err := gz.Close(); err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(buf.Bytes())
	checksum := hex.EncodeToString(sum[:])
	_, err := c.minio.PutObject(ctx, c.bucket, objectKey, bytes.NewReader(buf.Bytes()), int64(buf.Len()), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return checksum, int64(buf.Len()), err
}

func addPath(tw *tar.Writer, root, abs string) error {
	_, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return filepath.Walk(abs, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(rel, "..") {
			return fmt.Errorf("cache path %s escapes workspace", path)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})
}

func (c *Client) DownloadArchive(ctx context.Context, objectKey, checksum, dest string) (int64, error) {
	obj, err := c.minio.GetObject(ctx, c.bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return 0, err
	}
	defer obj.Close()
	payload, err := io.ReadAll(obj)
	if err != nil {
		return 0, err
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != checksum {
		return 0, errors.New("cache checksum mismatch")
	}
	gr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return 0, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	if err := SafeExtract(tr, dest); err != nil {
		return 0, err
	}
	return int64(len(payload)), nil
}

func SafeExtract(tr *tar.Reader, dest string) error {
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(cleanDest, header.Name)
		cleanTarget, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if cleanTarget != cleanDest && !strings.HasPrefix(cleanTarget, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("archive path traversal rejected: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(cleanTarget, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(cleanTarget), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(cleanTarget, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry type %d", header.Typeflag)
		}
	}
}

func Expiry(ttl time.Duration) *time.Time {
	if ttl <= 0 {
		return nil
	}
	t := time.Now().Add(ttl)
	return &t
}
