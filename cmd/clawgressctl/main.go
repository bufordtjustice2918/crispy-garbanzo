package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "configure":
		runConfigure(os.Args[2:])
	case "commit":
		runCommit(os.Args[2:])
	case "state":
		runState(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func runConfigure(args []string) {
	fs := flag.NewFlagSet("configure", flag.ExitOnError)
	apiURL := fs.String("url", "http://127.0.0.1:8080", "admin API base URL")
	actor := fs.String("actor", "cli", "operator identity")
	file := fs.String("file", "", "JSON file with staged changes")
	fs.Parse(args)

	if *file == "" {
		fatal("--file is required")
	}

	changesData, err := os.ReadFile(*file)
	if err != nil {
		fatalf("read --file: %v", err)
	}

	var changes map[string]any
	if err := json.Unmarshal(changesData, &changes); err != nil {
		fatalf("parse changes JSON: %v", err)
	}

	body := map[string]any{
		"actor":   *actor,
		"changes": changes,
	}
	resp := doJSON(http.MethodPost, *apiURL+"/v1/opmode/configure", body)
	prettyPrint(resp)
}

func runCommit(args []string) {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	apiURL := fs.String("url", "http://127.0.0.1:8080", "admin API base URL")
	actor := fs.String("actor", "cli", "operator identity")
	expected := fs.String("expected-revision", "", "optional expected staged revision ID")
	fs.Parse(args)

	body := map[string]any{"actor": *actor}
	if *expected != "" {
		body["expected_revision_id"] = *expected
	}

	resp := doJSON(http.MethodPost, *apiURL+"/v1/opmode/commit", body)
	prettyPrint(resp)
}

func runState(args []string) {
	fs := flag.NewFlagSet("state", flag.ExitOnError)
	apiURL := fs.String("url", "http://127.0.0.1:8080", "admin API base URL")
	fs.Parse(args)
	resp := doJSON(http.MethodGet, *apiURL+"/v1/opmode/state", nil)
	prettyPrint(resp)
}

func doJSON(method, url string, payload any) map[string]any {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			fatalf("marshal request: %v", err)
		}
		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fatalf("build request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		fatalf("read response: %v", err)
	}

	out := map[string]any{"http_status": resp.StatusCode}
	if len(respData) > 0 {
		var parsed map[string]any
		if err := json.Unmarshal(respData, &parsed); err == nil {
			out["response"] = parsed
		} else {
			out["raw_response"] = string(respData)
		}
	}

	if resp.StatusCode >= 400 {
		prettyPrint(out)
		os.Exit(1)
	}
	return out
}

func prettyPrint(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatalf("encode output: %v", err)
	}
	fmt.Println(string(data))
}

func usage() {
	fmt.Println("clawgressctl <configure|commit|state> [flags]")
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fatal(fmt.Sprintf(format, args...))
}
