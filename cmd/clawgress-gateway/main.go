// clawgress-gateway: explicit HTTP/HTTPS CONNECT egress proxy.
//
// Agents set HTTP_PROXY=http://localhost:3128 (or HTTPS_PROXY).
// Identity is extracted from Proxy-Authorization: Basic base64(agent_id:api_key).
// Every request is policy-evaluated and audit-logged before forwarding.
//
// Environment:
//
//	CLAWGRESS_PROXY_LISTEN   listen address (default :3128)
//	CLAWGRESS_AGENTS_FILE    identity registry JSON (default /etc/clawgress/agents.json)
//	CLAWGRESS_POLICY_FILE    policy rules JSON      (default /etc/clawgress/policy.json)
//	CLAWGRESS_AUDIT_FILE     audit JSONL path       (default /var/log/clawgress/audit.jsonl)
package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/audit"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/identity"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/quota"
)

var reqSeq uint64

func main() {
	listenAddr := getenv("CLAWGRESS_PROXY_LISTEN", ":3128")
	agentsFile := getenv("CLAWGRESS_AGENTS_FILE", "/etc/clawgress/agents.json")
	policyFile := getenv("CLAWGRESS_POLICY_FILE", "/etc/clawgress/policy.json")
	quotaFile := getenv("CLAWGRESS_QUOTA_FILE", "/etc/clawgress/quotas.json")
	auditFile := getenv("CLAWGRESS_AUDIT_FILE", "/var/log/clawgress/audit.jsonl")
	jwtSecret := getenv("CLAWGRESS_JWT_SECRET", "")

	reg, err := identity.NewRegistry(agentsFile)
	if err != nil {
		log.Fatalf("load identity registry: %v", err)
	}

	eng, err := policy.NewEngine(policyFile)
	if err != nil {
		log.Fatalf("load policy engine: %v", err)
	}

	lim, err := quota.NewLimiter(quotaFile)
	if err != nil {
		log.Fatalf("load quota limiter: %v", err)
	}

	alog, err := audit.NewLog(auditFile)
	if err != nil {
		log.Fatalf("open audit log: %v", err)
	}
	defer alog.Close()

	// SIGHUP reloads identity and policy from disk without restart.
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP)
		for range ch {
			log.Println("SIGHUP: reloading identity, policy, and quotas")
			if err := reg.Load(); err != nil {
				log.Printf("reload identity: %v", err)
			}
			if err := eng.Load(); err != nil {
				log.Printf("reload policy: %v", err)
			}
			if err := lim.Load(); err != nil {
				log.Printf("reload quotas: %v", err)
			}
		}
	}()

	h := &proxyHandler{reg: reg, eng: eng, lim: lim, alog: alog, jwtSecret: []byte(jwtSecret)}
	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      h,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 0, // tunnels must not time out writes
	}

	log.Printf("clawgress-gateway listening on %s (agents=%s policy=%s quotas=%s audit=%s)",
		listenAddr, agentsFile, policyFile, quotaFile, auditFile)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Proxy handler
// ---------------------------------------------------------------------------

type proxyHandler struct {
	reg       *identity.Registry
	eng       *policy.Engine
	lim       *quota.Limiter
	alog      *audit.Log
	jwtSecret []byte
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := newRequestID()

	agentID, apiKey, bearerToken := extractProxyAuth(r)

	var ag *identity.Agent
	if apiKey != "" {
		ag = h.reg.LookupByKey(apiKey)
	}
	// Fall back to JWT Bearer token if no valid API key.
	if ag == nil && bearerToken != "" && len(h.jwtSecret) > 0 {
		if claims, err := identity.VerifyJWT(bearerToken, h.jwtSecret); err == nil {
			agentID = claims.AgentID
			// Build a synthetic Agent from JWT claims for the request lifecycle.
			ag = &identity.Agent{
				AgentID:     claims.AgentID,
				TeamID:      claims.TeamID,
				ProjectID:   claims.ProjectID,
				Environment: claims.Environment,
				Status:      "active",
			}
		}
	}

	dest := requestHost(r)

	// --- Identity check ---
	if ag == nil {
		h.writeAudit(audit.Event{
			RequestID:   reqID,
			AgentID:     agentID, // may be empty string if no header at all
			Destination: dest,
			Method:      r.Method,
			Decision:    "deny",
			PolicyID:    "no-identity",
			LatencyMs:   time.Since(start).Milliseconds(),
		})
		w.Header().Set("Proxy-Authenticate", `Basic realm="clawgress"`)
		http.Error(w, "407 Proxy Authentication Required — no valid identity", http.StatusProxyAuthRequired)
		return
	}

	// --- Quota check ---
	qd := h.lim.Check(ag.AgentID)
	if !qd.Allowed {
		h.writeAudit(audit.Event{
			RequestID:   reqID,
			AgentID:     ag.AgentID,
			TeamID:      ag.TeamID,
			ProjectID:   ag.ProjectID,
			Environment: ag.Environment,
			Destination: dest,
			Method:      r.Method,
			Decision:    "deny",
			PolicyID:    "quota-exceeded",
			LatencyMs:   time.Since(start).Milliseconds(),
		})
		http.Error(w, fmt.Sprintf("429 Too Many Requests — %s", qd.Reason), http.StatusTooManyRequests)
		return
	}
	if qd.Reason != "" {
		// alert_only mode: log but continue
		log.Printf("quota alert: agent=%s %s", ag.AgentID, qd.Reason)
	}

	// --- Policy check ---
	dec := h.eng.Evaluate(ag.AgentID, dest)
	if dec.Action != "allow" {
		h.writeAudit(audit.Event{
			RequestID:   reqID,
			AgentID:     ag.AgentID,
			TeamID:      ag.TeamID,
			ProjectID:   ag.ProjectID,
			Environment: ag.Environment,
			Destination: dest,
			Method:      r.Method,
			Decision:    "deny",
			PolicyID:    dec.PolicyID,
			LatencyMs:   time.Since(start).Milliseconds(),
		})
		http.Error(w, fmt.Sprintf("403 Forbidden — %s", dec.Reason), http.StatusForbidden)
		return
	}

	// --- Forward ---
	if r.Method == http.MethodConnect {
		h.handleConnect(w, r, ag, reqID, start, dec)
	} else {
		h.handleHTTP(w, r, ag, reqID, start, dec)
	}
}

// handleConnect tunnels HTTPS (and any CONNECT) traffic.
func (h *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request,
	ag *identity.Agent, reqID string, start time.Time, dec policy.Decision) {

	upstream, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, "502 Bad Gateway — upstream unreachable", http.StatusBadGateway)
		h.writeAudit(audit.Event{
			RequestID: reqID, AgentID: ag.AgentID, TeamID: ag.TeamID,
			ProjectID: ag.ProjectID, Environment: ag.Environment,
			Destination: r.Host, Method: r.Method,
			Decision: "allow-upstream-error", PolicyID: dec.PolicyID,
			LatencyMs: time.Since(start).Milliseconds(),
		})
		return
	}
	defer upstream.Close()

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "500 Internal Server Error — hijack unsupported", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Signal tunnel established.
	fmt.Fprint(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

	// Bidirectional copy until either side closes.
	done := make(chan struct{}, 2)
	var bytesOut int64
	go func() {
		n, _ := io.Copy(upstream, clientConn)
		atomic.AddInt64(&bytesOut, n)
		upstream.Close()
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, upstream)
		clientConn.Close()
		done <- struct{}{}
	}()
	<-done // wait for first half-close; the deferred closes clean up the other

	h.writeAudit(audit.Event{
		RequestID: reqID, AgentID: ag.AgentID, TeamID: ag.TeamID,
		ProjectID: ag.ProjectID, Environment: ag.Environment,
		Destination: r.Host, Method: r.Method,
		Decision: "allow", PolicyID: dec.PolicyID,
		LatencyMs: time.Since(start).Milliseconds(),
		BytesOut:  atomic.LoadInt64(&bytesOut),
	})
}

// handleHTTP forwards plain HTTP requests.
func (h *proxyHandler) handleHTTP(w http.ResponseWriter, r *http.Request,
	ag *identity.Agent, reqID string, start time.Time, dec policy.Decision) {

	// Strip proxy-specific headers before forwarding.
	r.Header.Del("Proxy-Authorization")
	r.Header.Del("Proxy-Connection")
	r.RequestURI = ""

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects on behalf of the client
		},
	}
	resp, err := client.Do(r)
	if err != nil {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	n, _ := io.Copy(w, resp.Body)

	h.writeAudit(audit.Event{
		RequestID: reqID, AgentID: ag.AgentID, TeamID: ag.TeamID,
		ProjectID: ag.ProjectID, Environment: ag.Environment,
		Destination: requestHost(r), Method: r.Method,
		Decision: "allow", PolicyID: dec.PolicyID,
		LatencyMs: time.Since(start).Milliseconds(),
		BytesOut:  n,
	})
}

func (h *proxyHandler) writeAudit(e audit.Event) {
	if err := h.alog.Write(e); err != nil {
		log.Printf("audit write error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// extractProxyAuth decodes Proxy-Authorization header.
// Supports Basic base64(agent_id:api_key) and Bearer <jwt>.
// Returns (agentID, apiKey, bearerToken). At most one of apiKey/bearerToken is non-empty.
func extractProxyAuth(r *http.Request) (agentID, apiKey, bearerToken string) {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		return "", "", ""
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return "", "", strings.TrimPrefix(auth, "Bearer ")
	}
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return "", "", ""
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, prefix))
	if err != nil {
		return "", "", ""
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return "", "", ""
	}
	return parts[0], parts[1], ""
}

func requestHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}
	if r.URL != nil {
		return r.URL.Host
	}
	return ""
}

func newRequestID() string {
	n := atomic.AddUint64(&reqSeq, 1)
	return fmt.Sprintf("req-%d-%04d", time.Now().UnixMilli(), n%10000)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
