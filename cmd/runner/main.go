package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/buildplane/buildplane/internal/domain"
)

type update struct {
	JobID         string           `json:"job_id"`
	AttemptID     string           `json:"attempt_id"`
	LeaseToken    string           `json:"lease_token"`
	Chunk         string           `json:"chunk,omitempty"`
	Status        domain.JobStatus `json:"status,omitempty"`
	ExitCode      int              `json:"exit_code,omitempty"`
	FailureReason string           `json:"failure_reason,omitempty"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	env := requiredEnv()
	if err := post(ctx, env, "/internal/attempts/start", update{}); err != nil {
		log.Fatal(err)
	}
	go heartbeat(ctx, env)
	exitCode, reason := run(ctx, env)
	status := domain.StatusSucceeded
	if exitCode != 0 {
		status = domain.StatusFailed
		if reason == "" {
			reason = "user_command_failed"
		}
	}
	if err := post(context.Background(), env, "/internal/attempts/complete", update{Status: status, ExitCode: exitCode, FailureReason: reason}); err != nil {
		log.Fatal(err)
	}
}

type runnerEnv struct {
	APIURL        string
	Token         string
	JobID         string
	AttemptID     string
	LeaseToken    string
	Repo          string
	Commit        string
	Commands      []string
	WorkspaceRoot string
}

func requiredEnv() runnerEnv {
	return runnerEnv{
		APIURL:        strings.TrimRight(os.Getenv("BUILDPLANE_API_URL"), "/"),
		Token:         os.Getenv("BUILDPLANE_INTERNAL_TOKEN"),
		JobID:         os.Getenv("BUILDPLANE_JOB_ID"),
		AttemptID:     os.Getenv("BUILDPLANE_ATTEMPT_ID"),
		LeaseToken:    os.Getenv("BUILDPLANE_LEASE_TOKEN"),
		Repo:          os.Getenv("BUILDPLANE_REPOSITORY_URL"),
		Commit:        os.Getenv("BUILDPLANE_COMMIT_SHA"),
		Commands:      splitCommands(os.Getenv("BUILDPLANE_COMMANDS")),
		WorkspaceRoot: "/workspace",
	}
}

func splitCommands(raw string) []string {
	lines := strings.Split(raw, "\n")
	var commands []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commands = append(commands, line)
		}
	}
	return commands
}

func heartbeat(ctx context.Context, env runnerEnv) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := post(ctx, env, "/internal/attempts/heartbeat", update{}); err != nil {
				log.Printf("heartbeat rejected: %v", err)
				os.Exit(2)
			}
		}
	}
}

func run(ctx context.Context, env runnerEnv) (int, string) {
	if env.APIURL == "" || env.JobID == "" || env.AttemptID == "" || env.LeaseToken == "" {
		return 2, "invalid_workflow"
	}
	workdir := filepath.Join(env.WorkspaceRoot, "repo")
	_ = os.RemoveAll(workdir)
	if err := os.MkdirAll(env.WorkspaceRoot, 0o755); err != nil {
		_ = appendLog(ctx, env, fmt.Sprintf("create workspace failed: %v\n", err))
		return 2, "infrastructure"
	}
	if strings.HasPrefix(env.Repo, "fixture://") {
		if err := createFixtureRepo(workdir); err != nil {
			_ = appendLog(ctx, env, fmt.Sprintf("fixture repo failed: %v\n", err))
			return 2, "invalid_repository"
		}
	} else {
		if err := runAndLog(ctx, env, env.WorkspaceRoot, "git", "clone", "--depth=1", env.Repo, workdir); err != nil {
			return exitCode(err), "invalid_repository"
		}
		if err := runAndLog(ctx, env, workdir, "git", "checkout", env.Commit); err != nil {
			return exitCode(err), "invalid_repository"
		}
	}
	for _, command := range env.Commands {
		_ = appendLog(ctx, env, "$ "+command+"\n")
		// The MVP uses shell execution because CI commands commonly rely on shell syntax.
		// Production should combine policy controls, secret isolation, and stronger sandboxes.
		if err := runAndLog(ctx, env, workdir, "/bin/sh", "-lc", command); err != nil {
			return exitCode(err), "user_command_failed"
		}
	}
	return 0, ""
}

func createFixtureRepo(workdir string) error {
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workdir, "go.mod"), []byte("module demo\n\ngo 1.24\n"), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workdir, "main.go"), []byte("package main\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workdir, "main_test.go"), []byte("package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad add\") } }\n"), 0o644)
}

func runAndLog(ctx context.Context, env runnerEnv, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	err := cmd.Run()
	if buf.Len() > 0 {
		_ = appendLog(ctx, env, buf.String())
	}
	return err
}

func exitCode(err error) int {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}

func appendLog(ctx context.Context, env runnerEnv, chunk string) error {
	return post(ctx, env, "/internal/attempts/logs", update{Chunk: chunk})
}

func post(ctx context.Context, env runnerEnv, path string, u update) error {
	u.JobID = env.JobID
	u.AttemptID = env.AttemptID
	u.LeaseToken = env.LeaseToken
	payload, _ := json.Marshal(u)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env.APIURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, string(body))
	}
	return nil
}
