package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/enforcer"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/opmode"
)

func main() {
	stateDir := getenv("CLAWGRESS_STATE_DIR", "state")
	listenAddr := getenv("CLAWGRESS_ADMIN_LISTEN", ":8080")
	nftApply := getenvBool("CLAWGRESS_NFT_APPLY", true)
	defaultOpsMode := getenv("CLAWGRESS_OPS_MODE", enforcer.OpsModeDryRun)

	store, err := opmode.NewStore(stateDir)
	if err != nil {
		log.Fatalf("initialize store: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

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

	log.Printf("clawgress-admin-api listening on %s (state dir: %s)", listenAddr, stateDir)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatalf("http server failed: %v", err)
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
