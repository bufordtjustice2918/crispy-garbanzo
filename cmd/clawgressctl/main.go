package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	case "set":
		runSet(os.Args[2:])
	case "show":
		runShow(os.Args[2:])
	case "install":
		runInstall(os.Args[2:])
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

func runSet(args []string) {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	file := fs.String("file", "candidate.json", "candidate configuration JSON file")
	fs.Parse(args)

	if fs.NArg() != 2 {
		fatal("usage: clawgressctl set [--file candidate.json] <dot.path> <value>")
	}

	keyPath := fs.Arg(0)
	value := parseValue(fs.Arg(1))

	cfg, err := loadOrInitConfig(*file)
	if err != nil {
		fatalf("load config: %v", err)
	}
	setByPath(cfg, keyPath, value)

	if err := writeJSONFile(*file, cfg); err != nil {
		fatalf("write config: %v", err)
	}

	fmt.Printf("set %s in %s\n", keyPath, *file)
}

func runShow(args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	file := fs.String("file", "candidate.json", "candidate configuration JSON file")
	key := fs.String("key", "", "optional dot.path key")
	fs.Parse(args)

	cfg, err := loadOrInitConfig(*file)
	if err != nil {
		fatalf("load config: %v", err)
	}

	if *key == "" {
		prettyPrint(cfg)
		return
	}

	v, ok := getByPath(cfg, *key)
	if !ok {
		fatalf("key not found: %s", *key)
	}
	prettyPrint(map[string]any{"key": *key, "value": v})
}

func runInstall(args []string) {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	targetDisk := fs.String("target-disk", "", "install target disk (example: /dev/sda)")
	hostname := fs.String("hostname", "clawgress", "target hostname")
	autoReboot := fs.Bool("reboot", false, "reboot after install")
	yes := fs.Bool("yes", false, "confirm install plan")
	fs.Parse(args)

	if *targetDisk == "" {
		fatal("--target-disk is required")
	}

	plan := map[string]any{
		"mode":                  "livecd-to-disk",
		"boot_source":           "squashfs",
		"target_disk":           *targetDisk,
		"hostname":              *hostname,
		"auto_reboot":           *autoReboot,
		"confirmed":             *yes,
		"status":                "planned",
		"requires_root":         true,
		"timestamp":             time.Now().UTC().Format(time.RFC3339),
		"next_step":             "sudo clawgress-install --apply <generated-plan>",
		"mvp_note":              "Installer execution is planned in MVP scope; this command currently emits a validated plan.",
		"transactional_profile": "commit/configure",
	}
	prettyPrint(plan)

	if !*yes {
		fmt.Println("pass --yes to confirm this installation plan")
	}
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

func loadOrInitConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, nil
}

func writeJSONFile(path string, v any) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func setByPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	m := root
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		next, ok := m[key]
		if !ok {
			child := map[string]any{}
			m[key] = child
			m = child
			continue
		}
		child, ok := next.(map[string]any)
		if !ok {
			child = map[string]any{}
			m[key] = child
		}
		m = child
	}
	m[parts[len(parts)-1]] = value
}

func getByPath(root map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = root
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := m[part]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func parseValue(raw string) any {
	if raw == "true" {
		return true
	}
	if raw == "false" {
		return false
	}
	if i, err := strconv.Atoi(raw); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			return v
		}
	}
	return raw
}

func prettyPrint(v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatalf("encode output: %v", err)
	}
	fmt.Println(string(data))
}

func usage() {
	fmt.Println("clawgressctl <configure|commit|state|set|show|install> [flags]")
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func fatalf(format string, args ...any) {
	fatal(fmt.Sprintf(format, args...))
}
