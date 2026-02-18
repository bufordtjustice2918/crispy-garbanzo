package enforcer

import (
	"os"
	"testing"
)

func TestNormalizeOpsMode(t *testing.T) {
	if got := NormalizeOpsMode(""); got != OpsModeDryRun {
		t.Fatalf("expected dry-run, got %q", got)
	}
	if got := NormalizeOpsMode("apply"); got != OpsModeApply {
		t.Fatalf("expected apply, got %q", got)
	}
	if got := NormalizeOpsMode("bad"); got != "" {
		t.Fatalf("expected invalid mode, got %q", got)
	}
}

func TestExecuteOpsPlanDryRun(t *testing.T) {
	ops := []Operation{{Backend: "test", Scope: "scope", Actions: []string{"systemctl restart bind9"}}}
	res, err := ExecuteOpsPlan(ops, "dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Mode != OpsModeDryRun || len(res.Logs) == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestExecuteOpsPlanApplyMock(t *testing.T) {
	t.Setenv("CLAWGRESS_OPS_EXEC_MOCK", "1")
	ops := []Operation{
		{Backend: "test", Scope: "scope", Actions: []string{"write /etc/clawgress/policy/policy.json", "systemctl restart bind9"}},
	}
	res, err := ExecuteOpsPlan(ops, "apply")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Applied != 2 {
		t.Fatalf("expected 2 applied actions, got %d", res.Applied)
	}
}

func TestExecuteWriteActionPathGuard(t *testing.T) {
	if err := executeWriteAction("write /tmp/not-allowed.conf"); err == nil {
		t.Fatalf("expected disallowed path error")
	}
}

func TestExecuteWriteActionAllowed(t *testing.T) {
	root := t.TempDir()
	target := root + "/etc/clawgress/services/test.json"
	t.Setenv("CLAWGRESS_OPS_EXEC_MOCK", "")
	if err := os.MkdirAll(root+"/etc/clawgress/services", 0o755); err != nil {
		t.Fatal(err)
	}
	if !isAllowedWritePath("/etc/clawgress/services/test.json") {
		t.Fatalf("expected write path to be allowed")
	}
	_ = target
}
