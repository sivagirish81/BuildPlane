package cache

import (
	"archive/tar"
	"bytes"
	"testing"
)

func TestKeyDeterministic(t *testing.T) {
	a, objA := Key(KeyInput{TenantID: "tenant-a", Prefix: "go", KeyFiles: map[string][]byte{"go.sum": []byte("abc")}, Commands: []string{"go test"}, Env: map[string]string{"A": "B"}})
	b, objB := Key(KeyInput{TenantID: "tenant-a", Prefix: "go", KeyFiles: map[string][]byte{"go.sum": []byte("abc")}, Commands: []string{"go test"}, Env: map[string]string{"A": "B"}})
	if a != b || objA != objB {
		t.Fatal("cache key should be deterministic")
	}
}

func TestSafeExtractRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: "../evil", Mode: 0o644, Size: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	if err := SafeExtract(tr, t.TempDir()); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
