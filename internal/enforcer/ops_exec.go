package enforcer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	OpsModeDryRun = "dry-run"
	OpsModeApply  = "apply"
)

type OpsExecResult struct {
	Mode    string   `json:"mode"`
	Applied int      `json:"applied"`
	Failed  int      `json:"failed"`
	Logs    []string `json:"logs"`
}

func NormalizeOpsMode(mode string) string {
	mode = strings.TrimSpace(strings.ToLower(mode))
	switch mode {
	case "", "dryrun", "dry-run":
		return OpsModeDryRun
	case "apply":
		return OpsModeApply
	default:
		return ""
	}
}

func ExecuteOpsPlan(ops []Operation, mode string) (OpsExecResult, error) {
	nm := NormalizeOpsMode(mode)
	if nm == "" {
		return OpsExecResult{}, fmt.Errorf("invalid ops mode: %q", mode)
	}
	out := OpsExecResult{
		Mode: nm,
		Logs: make([]string, 0),
	}
	for _, op := range ops {
		for _, action := range op.Actions {
			line := fmt.Sprintf("%s %s: %s", op.Backend, op.Scope, action)
			if nm == OpsModeDryRun {
				out.Logs = append(out.Logs, "DRYRUN "+line)
				continue
			}
			if err := executeAction(action); err != nil {
				out.Failed++
				out.Logs = append(out.Logs, "ERROR "+line+" :: "+err.Error())
				return out, err
			}
			out.Applied++
			out.Logs = append(out.Logs, "APPLIED "+line)
		}
	}
	return out, nil
}

func executeAction(action string) error {
	trimmed := strings.TrimSpace(action)
	if trimmed == "" {
		return nil
	}
	if os.Getenv("CLAWGRESS_OPS_EXEC_MOCK") == "1" {
		return nil
	}
	switch {
	case strings.HasPrefix(trimmed, "write "):
		return executeWriteAction(trimmed)
	case strings.HasPrefix(trimmed, "systemctl "):
		return runCommand("systemctl", strings.Fields(trimmed)[1:]...)
	case strings.HasPrefix(trimmed, "ip "):
		return runCommand("ip", strings.Fields(trimmed)[1:]...)
	case strings.HasPrefix(trimmed, "nft "):
		return runCommand("nft", strings.Fields(trimmed)[1:]...)
	default:
		return fmt.Errorf("action not allowed: %s", trimmed)
	}
}

func executeWriteAction(action string) error {
	pathAndMeta := strings.TrimSpace(strings.TrimPrefix(action, "write "))
	path := pathAndMeta
	if idx := strings.Index(pathAndMeta, " "); idx > 0 {
		path = pathAndMeta[:idx]
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("empty write path")
	}
	if !isAllowedWritePath(path) {
		return fmt.Errorf("write path not allowed: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	content := fmt.Sprintf("# managed by clawgress ops executor\n# %s\n", time.Now().UTC().Format(time.RFC3339))
	return os.WriteFile(path, []byte(content), 0o644)
}

func isAllowedWritePath(path string) bool {
	allowed := []string{
		"/etc/clawgress/",
		"/etc/systemd/",
		"/run/dhclient/",
		"/run/systemd/",
		"/etc/bind/",
		"/etc/haproxy/",
		"/etc/ssh/",
		"/etc/nftables.d/",
		"/etc/chrony/",
	}
	for _, p := range allowed {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed: %w (%s)", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
