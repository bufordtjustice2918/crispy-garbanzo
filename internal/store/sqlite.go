// Package store provides a SQLite-backed persistence layer for agents, policies, and quotas.
//
// The Store wraps identity.Registry, policy.Engine, and quota.Limiter with
// database persistence. On startup it loads data from SQLite into the in-memory
// structures. Writes go to both SQLite and the in-memory maps atomically.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/bufordtjustice2918/crispy-garbanzo/internal/identity"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/policy"
	"github.com/bufordtjustice2918/crispy-garbanzo/internal/quota"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection and provides CRUD for agents, policies, quotas.
type DB struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// WAL mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &DB{db: db}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			agent_id    TEXT PRIMARY KEY,
			team_id     TEXT NOT NULL DEFAULT '',
			project_id  TEXT NOT NULL DEFAULT '',
			environment TEXT NOT NULL DEFAULT '',
			api_key     TEXT NOT NULL,
			status      TEXT NOT NULL DEFAULT 'active'
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_api_key ON agents(api_key)`,
		`CREATE TABLE IF NOT EXISTS policies (
			policy_id TEXT PRIMARY KEY,
			agent_id  TEXT NOT NULL DEFAULT '*',
			domains   TEXT NOT NULL DEFAULT '[]',
			action    TEXT NOT NULL DEFAULT 'deny',
			priority  INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS quotas (
			agent_id TEXT PRIMARY KEY,
			rps      REAL NOT NULL DEFAULT 0,
			rpm      REAL NOT NULL DEFAULT 0,
			mode     TEXT NOT NULL DEFAULT 'hard_stop'
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

// LoadAgents reads all agents from SQLite into the Registry.
func (d *DB) LoadAgents(reg *identity.Registry) error {
	rows, err := d.db.Query("SELECT agent_id, team_id, project_id, environment, api_key, status FROM agents")
	if err != nil {
		return fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var a identity.Agent
		if err := rows.Scan(&a.AgentID, &a.TeamID, &a.ProjectID, &a.Environment, &a.APIKey, &a.Status); err != nil {
			return fmt.Errorf("scan agent: %w", err)
		}
		reg.Add(a)
		count++
	}
	if count > 0 {
		log.Printf("sqlite: loaded %d agents", count)
	}
	return rows.Err()
}

// SaveAgent upserts a single agent.
func (d *DB) SaveAgent(a identity.Agent) error {
	_, err := d.db.Exec(
		`INSERT INTO agents (agent_id, team_id, project_id, environment, api_key, status)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
		   team_id=excluded.team_id, project_id=excluded.project_id,
		   environment=excluded.environment, api_key=excluded.api_key, status=excluded.status`,
		a.AgentID, a.TeamID, a.ProjectID, a.Environment, a.APIKey, a.Status,
	)
	return err
}

// DeleteAgent removes an agent by ID.
func (d *DB) DeleteAgent(agentID string) error {
	_, err := d.db.Exec("DELETE FROM agents WHERE agent_id = ?", agentID)
	return err
}

// ---------------------------------------------------------------------------
// Policies
// ---------------------------------------------------------------------------

// LoadPolicies reads all policies from SQLite into the Engine.
func (d *DB) LoadPolicies(eng *policy.Engine) error {
	rows, err := d.db.Query("SELECT policy_id, agent_id, domains, action FROM policies ORDER BY priority, rowid")
	if err != nil {
		return fmt.Errorf("query policies: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var r policy.Rule
		var domainsJSON string
		if err := rows.Scan(&r.PolicyID, &r.AgentID, &domainsJSON, &r.Action); err != nil {
			return fmt.Errorf("scan policy: %w", err)
		}
		// Domains stored as JSON array string.
		if err := json.Unmarshal([]byte(domainsJSON), &r.Domains); err != nil {
			r.Domains = []string{}
		}
		eng.Add(r)
		count++
	}
	if count > 0 {
		log.Printf("sqlite: loaded %d policies", count)
	}
	return rows.Err()
}

// SavePolicy upserts a single policy.
func (d *DB) SavePolicy(r policy.Rule) error {
	domainsJSON, err := json.Marshal(r.Domains)
	if err != nil {
		return fmt.Errorf("marshal domains: %w", err)
	}
	_, err = d.db.Exec(
		`INSERT INTO policies (policy_id, agent_id, domains, action)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(policy_id) DO UPDATE SET
		   agent_id=excluded.agent_id, domains=excluded.domains, action=excluded.action`,
		r.PolicyID, r.AgentID, string(domainsJSON), r.Action,
	)
	return err
}

// DeletePolicy removes a policy by ID.
func (d *DB) DeletePolicy(policyID string) error {
	_, err := d.db.Exec("DELETE FROM policies WHERE policy_id = ?", policyID)
	return err
}

// ---------------------------------------------------------------------------
// Quotas
// ---------------------------------------------------------------------------

// LoadQuotas reads all quotas from SQLite into the Limiter.
func (d *DB) LoadQuotas(lim *quota.Limiter) error {
	rows, err := d.db.Query("SELECT agent_id, rps, rpm, mode FROM quotas")
	if err != nil {
		return fmt.Errorf("query quotas: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var q quota.Limit
		if err := rows.Scan(&q.AgentID, &q.RPS, &q.RPM, &q.Mode); err != nil {
			return fmt.Errorf("scan quota: %w", err)
		}
		lim.Set(q)
		count++
	}
	if count > 0 {
		log.Printf("sqlite: loaded %d quotas", count)
	}
	return rows.Err()
}

// SaveQuota upserts a single quota.
func (d *DB) SaveQuota(q quota.Limit) error {
	_, err := d.db.Exec(
		`INSERT INTO quotas (agent_id, rps, rpm, mode)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(agent_id) DO UPDATE SET
		   rps=excluded.rps, rpm=excluded.rpm, mode=excluded.mode`,
		q.AgentID, q.RPS, q.RPM, q.Mode,
	)
	return err
}

// DeleteQuota removes a quota by agent ID.
func (d *DB) DeleteQuota(agentID string) error {
	_, err := d.db.Exec("DELETE FROM quotas WHERE agent_id = ?", agentID)
	return err
}
