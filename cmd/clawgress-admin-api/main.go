package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/enforcer"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/identity"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/opmode"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
)

func main() {
	stateDir := getenv("CLAWGRESS_STATE_DIR", "state")
	listenAddr := getenv("CLAWGRESS_ADMIN_LISTEN", ":8080")
	nftApply := getenvBool("CLAWGRESS_NFT_APPLY", true)
	defaultOpsMode := getenv("CLAWGRESS_OPS_MODE", enforcer.OpsModeDryRun)
	agentsFile := getenv("CLAWGRESS_AGENTS_FILE", "/etc/clawgress/agents.json")
	policyFile := getenv("CLAWGRESS_POLICY_FILE", "/etc/clawgress/policy.json")

	store, err := opmode.NewStore(stateDir)
	if err != nil {
		log.Fatalf("initialize store: %v", err)
	}

	reg, err := identity.NewRegistry(agentsFile)
	if err != nil {
		log.Fatalf("load identity registry: %v", err)
	}

	eng, err := policy.NewEngine(policyFile)
	if err != nil {
		log.Fatalf("load policy engine: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// -----------------------------------------------------------------------
	// Opmode endpoints (existing)
	// -----------------------------------------------------------------------

	mux.HandleFunc("/v1/opmode/configure", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req opmode.ConfigureRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if req.Actor == "" {
			req.Actor = "unknown"
		}
		if req.Changes == nil {
			req.Changes = map[string]any{}
		}

		resp, err := store.Configure(req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/v1/opmode/commit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		var req opmode.CommitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		if req.Actor == "" {
			req.Actor = "unknown"
		}

		resp, err := store.Commit(req)
		if err != nil {
			switch {
			case errors.Is(err, opmode.ErrNoStagedRevision):
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			case errors.Is(err, opmode.ErrRevisionMismatch):
				writeJSON(w, http.StatusPreconditionFailed, map[string]string{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}
		state, err := store.State()
		if err != nil {
			resp.NftApply = "error"
			resp.NftError = err.Error()
			writeJSON(w, http.StatusInternalServerError, resp)
			return
		}
		if state.Active != nil {
			ops := enforcer.BuildOpsPlan(state.Active.Changes)
			resp.OpsPlan = ops
			mode := req.OpsMode
			if mode == "" {
				mode = defaultOpsMode
			}
			resp.OpsMode = enforcer.NormalizeOpsMode(mode)
			if resp.OpsMode == "" {
				resp.OpsMode = enforcer.OpsModeDryRun
			}
			opsResult, err := enforcer.ExecuteOpsPlan(ops, resp.OpsMode)
			if err != nil {
				resp.OpsStatus = "error"
				resp.OpsError = err.Error()
				writeJSON(w, http.StatusInternalServerError, resp)
				return
			}
			resp.OpsStatus = opsResult.Mode
		}

		if nftApply {
			if state.Active == nil {
				resp.NftApply = "error"
				resp.NftError = "active revision missing after commit"
				writeJSON(w, http.StatusInternalServerError, resp)
				return
			}
			applyResult, err := enforcer.ApplyNftables(stateDir, *state.Active, true)
			if err != nil {
				resp.NftApply = "error"
				resp.NftError = err.Error()
				writeJSON(w, http.StatusInternalServerError, resp)
				return
			}
			resp.NftApply = "applied"
			resp.NftRules = applyResult.RulesPath
		} else {
			resp.NftApply = "disabled"
		}
		writeJSON(w, http.StatusOK, resp)
	})

	mux.HandleFunc("/v1/opmode/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		state, err := store.State()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, state)
	})

	// -----------------------------------------------------------------------
	// Agent CRUD endpoints
	// -----------------------------------------------------------------------

	mux.HandleFunc("/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, reg.All())
		case http.MethodPost:
			var a identity.Agent
			if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
			if a.AgentID == "" || a.APIKey == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent_id and api_key are required"})
				return
			}
			if a.Status == "" {
				a.Status = "active"
			}
			reg.Add(a)
			if err := reg.Save(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			signalGateway()
			writeJSON(w, http.StatusCreated, a)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	mux.HandleFunc("/v1/agents/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/agents/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "agent_id required in path"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			a := reg.LookupByID(id)
			if a == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
				return
			}
			writeJSON(w, http.StatusOK, a)
		case http.MethodDelete:
			if !reg.Remove(id) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
				return
			}
			if err := reg.Save(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			signalGateway()
			writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	// -----------------------------------------------------------------------
	// Policy CRUD endpoints
	// -----------------------------------------------------------------------

	mux.HandleFunc("/v1/policies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, eng.Rules())
		case http.MethodPost:
			var rule policy.Rule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
				return
			}
			if rule.PolicyID == "" || rule.Action == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy_id and action are required"})
				return
			}
			if rule.Action != "allow" && rule.Action != "deny" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action must be 'allow' or 'deny'"})
				return
			}
			if rule.AgentID == "" {
				rule.AgentID = "*"
			}
			eng.Add(rule)
			if err := eng.Save(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			signalGateway()
			writeJSON(w, http.StatusCreated, rule)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	mux.HandleFunc("/v1/policies/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/v1/policies/")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy_id required in path"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			rule := eng.LookupByID(id)
			if rule == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
				return
			}
			writeJSON(w, http.StatusOK, rule)
		case http.MethodDelete:
			if !eng.Remove(id) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
				return
			}
			if err := eng.Save(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			signalGateway()
			writeJSON(w, http.StatusOK, map[string]string{"deleted": id})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	log.Printf("clawgress-admin-api listening on %s (state dir: %s, agents: %s, policy: %s)",
		listenAddr, stateDir, agentsFile, policyFile)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("http server failed: %v", err)
	}
}

// signalGateway sends SIGHUP to the gateway process so it reloads identity and policy.
func signalGateway() {
	out, err := exec.Command("pidof", "clawgress-gateway").Output()
	if err != nil {
		log.Printf("signalGateway: gateway not running (pidof: %v)", err)
		return
	}
	pidStr := strings.TrimSpace(string(out))
	if pidStr == "" {
		return
	}
	// pidof may return multiple PIDs; signal all of them.
	for _, s := range strings.Fields(pidStr) {
		pid, err := strconv.Atoi(s)
		if err != nil {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			if err := p.Signal(syscall.SIGHUP); err != nil {
				log.Printf("signalGateway: SIGHUP pid %d: %v", pid, err)
			} else {
				log.Printf("signalGateway: sent SIGHUP to gateway pid %d", pid)
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
