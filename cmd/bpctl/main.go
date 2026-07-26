package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {
	api := getenv("BUILDPLANE_API_URL", "http://localhost:8080")
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	switch {
	case match(args, "workflow", "create"):
		file := flag(args, "-f", "")
		if file == "" {
			log.Fatal("workflow create requires -f")
		}
		payload := readYAMLAsJSON(file)
		do(api, http.MethodPost, "/v1/workflows", "", payload, os.Stdout)
	case match(args, "run", "create"):
		body := map[string]any{
			"workflow_id":    flag(args, "--workflow", ""),
			"repository_url": flag(args, "--repo", ""),
			"commit_sha":     flag(args, "--commit", "main"),
			"priority":       flag(args, "--priority", "normal"),
		}
		payload, _ := json.Marshal(body)
		key := flag(args, "--idempotency-key", fmt.Sprintf("bpctl-%d", time.Now().UnixNano()))
		do(api, http.MethodPost, "/v1/runs", key, payload, os.Stdout)
	case match(args, "run", "get"):
		requireLen(args, 3)
		do(api, http.MethodGet, "/v1/runs/"+args[2], "", nil, os.Stdout)
	case match(args, "run", "watch"):
		requireLen(args, 3)
		for {
			var buf bytes.Buffer
			do(api, http.MethodGet, "/v1/runs/"+args[2], "", nil, &buf)
			fmt.Print(buf.String())
			if bytes.Contains(buf.Bytes(), []byte(`"Status":"SUCCEEDED"`)) || bytes.Contains(buf.Bytes(), []byte(`"Status":"FAILED"`)) || bytes.Contains(buf.Bytes(), []byte(`"Status":"CANCELLED"`)) {
				return
			}
			time.Sleep(2 * time.Second)
		}
	case match(args, "run", "cancel"):
		requireLen(args, 3)
		do(api, http.MethodPost, "/v1/runs/"+args[2]+"/cancel", "", []byte("{}"), os.Stdout)
	case match(args, "job", "logs"):
		requireLen(args, 3)
		do(api, http.MethodGet, "/v1/jobs/"+args[2]+"/logs", "", nil, os.Stdout)
	case match(args, "queue", "status"):
		fmt.Println("Queue status is exposed through Prometheus metrics at /metrics: buildplane_queue_depth and buildplane_queue_oldest_age_seconds.")
	default:
		usage()
		os.Exit(2)
	}
}

func readYAMLAsJSON(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		log.Fatal(err)
	}
	normalized := normalize(v)
	payload, err := json.Marshal(normalized)
	if err != nil {
		log.Fatal(err)
	}
	return payload
}

func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range x {
			out[k] = normalize(v)
		}
		return out
	case []any:
		for i := range x {
			x[i] = normalize(x[i])
		}
	}
	return v
}

func do(api, method, path, idempotencyKey string, payload []byte, out io.Writer) {
	req, err := http.NewRequest(method, api+path, bytes.NewReader(payload))
	if err != nil {
		log.Fatal(err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(out, resp.Body)
	if resp.StatusCode >= 300 {
		os.Exit(1)
	}
}

func match(args []string, a, b string) bool {
	return len(args) >= 2 && args[0] == a && args[1] == b
}

func flag(args []string, name, fallback string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return fallback
}

func requireLen(args []string, n int) {
	if len(args) < n {
		usage()
		os.Exit(2)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func usage() {
	fmt.Println(`bpctl commands:
  bpctl workflow create -f workflow.yaml
  bpctl run create --workflow <id> --repo <url> --commit <sha> --priority high [--idempotency-key key]
  bpctl run get <run-id>
  bpctl run watch <run-id>
  bpctl run cancel <run-id>
  bpctl job logs <job-id>
  bpctl queue status`)
}
