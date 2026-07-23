package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type SQLiteRepository struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	repo := &SQLiteRepository{db: db}
	if err := repo.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *SQLiteRepository) Close() error { return r.db.Close() }

func (r *SQLiteRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

func (r *SQLiteRepository) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`CREATE TABLE IF NOT EXISTS admin_credentials (
			id INTEGER PRIMARY KEY CHECK (id=1),
			token_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS deployment_plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			package_id TEXT,
			image TEXT,
			remote_endpoint TEXT,
			transport TEXT,
			env_var_names TEXT NOT NULL,
			secret_references TEXT NOT NULL,
			cpu_limit TEXT,
			memory_limit TEXT,
			pid_limit INTEGER NOT NULL DEFAULT 0,
			host_network INTEGER NOT NULL DEFAULT 0,
			privileged INTEGER NOT NULL DEFAULT 0,
			host_mounts TEXT NOT NULL,
			broad_egress INTEGER NOT NULL DEFAULT 0,
			expose_public INTEGER NOT NULL DEFAULT 0,
			requested_tools TEXT NOT NULL,
			risk_findings TEXT NOT NULL,
			labels TEXT NOT NULL,
			requires_approval INTEGER NOT NULL DEFAULT 0,
			approved_by_id TEXT,
			approval_created_at TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS approvals (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(plan_id)
		);`,
		`CREATE TABLE IF NOT EXISTS installations (
			id TEXT PRIMARY KEY,
			plan_id TEXT NOT NULL,
			package_id TEXT,
			runtime_ref TEXT,
			state TEXT NOT NULL,
			image_digest TEXT,
			rollback_metadata TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			installation_ids TEXT NOT NULL,
			tool_allowlist TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS clients (
			id TEXT PRIMARY KEY,
			profile_id TEXT NOT NULL,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			revoked INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS secret_references (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			secret_cipher TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id TEXT PRIMARY KEY,
			actor TEXT NOT NULL,
			action TEXT NOT NULL,
			target TEXT NOT NULL,
			result TEXT NOT NULL,
			metadata TEXT NOT NULL,
			redacted INTEGER NOT NULL DEFAULT 1,
			request_id TEXT,
			remote_addr TEXT,
			profile_id TEXT,
			client_id TEXT,
			occurred_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (r *SQLiteRepository) GetAdminHash(ctx context.Context) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, `SELECT token_hash FROM admin_credentials WHERE id=1`).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return hash, err
}

func (r *SQLiteRepository) SetAdminHash(ctx context.Context, hash string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_credentials (id, token_hash, created_at, updated_at)
		VALUES (1, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET token_hash=excluded.token_hash, updated_at=excluded.updated_at
	`, hash, now, now)
	return err
}

func (r *SQLiteRepository) CreateDeploymentPlan(ctx context.Context, plan domain.DeploymentPlan) (domain.DeploymentPlan, error) {
	plan.CreatedAt = plan.CreatedAt.UTC()
	_, err := r.db.ExecContext(ctx, `INSERT INTO deployment_plans (
		id,name,package_id,image,remote_endpoint,transport,env_var_names,secret_references,cpu_limit,memory_limit,pid_limit,
		host_network,privileged,host_mounts,broad_egress,expose_public,requested_tools,risk_findings,labels,requires_approval,
		approved_by_id,approval_created_at,created_at
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		plan.ID, plan.Name, plan.PackageID, plan.Image, plan.RemoteEndpoint, plan.Transport,
		mustJSON(plan.EnvVarNames), mustJSON(plan.SecretReferences), plan.CPULimit, plan.MemoryLimit, plan.PIDLimit,
		boolInt(plan.HostNetwork), boolInt(plan.Privileged), mustJSON(plan.HostMounts), boolInt(plan.BroadEgress), boolInt(plan.ExposePublic),
		mustJSON(plan.RequestedTools), mustJSON(plan.RiskFindings), mustJSON(plan.Labels), boolInt(plan.RequiresApproval),
		plan.ApprovedByID, nullTime(plan.ApprovalCreatedAt), plan.CreatedAt.Format(time.RFC3339Nano),
	)
	return plan, err
}

func (r *SQLiteRepository) GetDeploymentPlan(ctx context.Context, id string) (domain.DeploymentPlan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id,name,package_id,image,remote_endpoint,transport,env_var_names,secret_references,cpu_limit,memory_limit,pid_limit,
		host_network,privileged,host_mounts,broad_egress,expose_public,requested_tools,risk_findings,labels,requires_approval,approved_by_id,approval_created_at,created_at
		FROM deployment_plans WHERE id=?`, id)
	return scanPlan(row)
}

func (r *SQLiteRepository) ListDeploymentPlans(ctx context.Context) ([]domain.DeploymentPlan, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,package_id,image,remote_endpoint,transport,env_var_names,secret_references,cpu_limit,memory_limit,pid_limit,
		host_network,privileged,host_mounts,broad_egress,expose_public,requested_tools,risk_findings,labels,requires_approval,approved_by_id,approval_created_at,created_at
		FROM deployment_plans ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var plans []domain.DeploymentPlan
	for rows.Next() {
		plan, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, rows.Err()
}

func scanPlan(scanner interface{ Scan(...any) error }) (domain.DeploymentPlan, error) {
	var p domain.DeploymentPlan
	var envJSON, secJSON, mountsJSON, toolsJSON, findingsJSON, labelsJSON string
	var hostNetwork, privileged, broadEgress, exposePublic, requiresApproval int
	var approval sql.NullString
	var approvedBy sql.NullString
	var created string
	if err := scanner.Scan(&p.ID, &p.Name, &p.PackageID, &p.Image, &p.RemoteEndpoint, &p.Transport, &envJSON, &secJSON, &p.CPULimit, &p.MemoryLimit,
		&p.PIDLimit, &hostNetwork, &privileged, &mountsJSON, &broadEgress, &exposePublic, &toolsJSON, &findingsJSON, &labelsJSON,
		&requiresApproval, &approvedBy, &approval, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return p, ErrNotFound
		}
		return p, err
	}
	_ = json.Unmarshal([]byte(envJSON), &p.EnvVarNames)
	_ = json.Unmarshal([]byte(secJSON), &p.SecretReferences)
	_ = json.Unmarshal([]byte(mountsJSON), &p.HostMounts)
	_ = json.Unmarshal([]byte(toolsJSON), &p.RequestedTools)
	_ = json.Unmarshal([]byte(findingsJSON), &p.RiskFindings)
	_ = json.Unmarshal([]byte(labelsJSON), &p.Labels)
	p.HostNetwork = hostNetwork == 1
	p.Privileged = privileged == 1
	p.BroadEgress = broadEgress == 1
	p.ExposePublic = exposePublic == 1
	p.RequiresApproval = requiresApproval == 1
	p.ApprovedByID = approvedBy.String
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	if approval.Valid {
		t, err := time.Parse(time.RFC3339Nano, approval.String)
		if err == nil {
			p.ApprovalCreatedAt = &t
		}
	}
	return p, nil
}

func (r *SQLiteRepository) CreateApproval(ctx context.Context, approval domain.Approval) (domain.Approval, error) {
	approval.CreatedAt = approval.CreatedAt.UTC()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return approval, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO approvals (id,plan_id,reason,created_at) VALUES (?,?,?,?)`, approval.ID, approval.PlanID, approval.Reason, approval.CreatedAt.Format(time.RFC3339Nano)); err != nil {
		return approval, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE deployment_plans SET approved_by_id=?, approval_created_at=?, requires_approval=0 WHERE id=?`, approval.ID, approval.CreatedAt.Format(time.RFC3339Nano), approval.PlanID); err != nil {
		return approval, err
	}
	if err := tx.Commit(); err != nil {
		return approval, err
	}
	return approval, nil
}

func (r *SQLiteRepository) GetApprovalByPlanID(ctx context.Context, planID string) (domain.Approval, bool, error) {
	var a domain.Approval
	var created string
	err := r.db.QueryRowContext(ctx, `SELECT id,plan_id,reason,created_at FROM approvals WHERE plan_id=?`, planID).Scan(&a.ID, &a.PlanID, &a.Reason, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return a, false, nil
	}
	if err != nil {
		return a, false, err
	}
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return a, true, nil
}

func (r *SQLiteRepository) ListApprovals(ctx context.Context) ([]domain.Approval, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,plan_id,reason,created_at FROM approvals ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Approval
	for rows.Next() {
		var a domain.Approval
		var created string
		if err := rows.Scan(&a.ID, &a.PlanID, &a.Reason, &created); err != nil {
			return nil, err
		}
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) CreateInstallation(ctx context.Context, in domain.Installation) (domain.Installation, error) {
	now := in.CreatedAt.UTC().Format(time.RFC3339Nano)
	updated := in.UpdatedAt.UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `INSERT INTO installations (id,plan_id,package_id,runtime_ref,state,image_digest,rollback_metadata,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		in.ID, in.PlanID, in.PackageID, in.RuntimeRef, in.State, in.ImageDigest, mustJSON(in.RollbackMetadata), now, updated)
	return in, err
}

func (r *SQLiteRepository) GetInstallation(ctx context.Context, id string) (domain.Installation, error) {
	var in domain.Installation
	var rollback, created, updated string
	err := r.db.QueryRowContext(ctx, `SELECT id,plan_id,package_id,runtime_ref,state,image_digest,rollback_metadata,created_at,updated_at FROM installations WHERE id=?`, id).
		Scan(&in.ID, &in.PlanID, &in.PackageID, &in.RuntimeRef, &in.State, &in.ImageDigest, &rollback, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return in, ErrNotFound
	}
	if err != nil {
		return in, err
	}
	_ = json.Unmarshal([]byte(rollback), &in.RollbackMetadata)
	in.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	in.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return in, nil
}

func (r *SQLiteRepository) ListInstallations(ctx context.Context) ([]domain.Installation, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,plan_id,package_id,runtime_ref,state,image_digest,rollback_metadata,created_at,updated_at FROM installations ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Installation
	for rows.Next() {
		var in domain.Installation
		var rollback, created, updated string
		if err := rows.Scan(&in.ID, &in.PlanID, &in.PackageID, &in.RuntimeRef, &in.State, &in.ImageDigest, &rollback, &created, &updated); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(rollback), &in.RollbackMetadata)
		in.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		in.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, in)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) UpdateInstallation(ctx context.Context, in domain.Installation) error {
	_, err := r.db.ExecContext(ctx, `UPDATE installations SET state=?, runtime_ref=?, image_digest=?, rollback_metadata=?, updated_at=? WHERE id=?`,
		in.State, in.RuntimeRef, in.ImageDigest, mustJSON(in.RollbackMetadata), in.UpdatedAt.UTC().Format(time.RFC3339Nano), in.ID)
	return err
}

func (r *SQLiteRepository) DisableInstallation(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE installations SET state='disabled', updated_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (r *SQLiteRepository) DeleteInstallation(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM installations WHERE id=?`, id)
	return err
}

func (r *SQLiteRepository) CreateProfile(ctx context.Context, p domain.Profile) (domain.Profile, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO profiles (id,name,installation_ids,tool_allowlist,created_at) VALUES (?,?,?,?,?)`,
		p.ID, p.Name, mustJSON(p.InstallationIDs), mustJSON(p.ToolAllowlist), p.CreatedAt.UTC().Format(time.RFC3339Nano))
	return p, err
}

func (r *SQLiteRepository) GetProfile(ctx context.Context, id string) (domain.Profile, error) {
	var p domain.Profile
	var ins, tools, created string
	err := r.db.QueryRowContext(ctx, `SELECT id,name,installation_ids,tool_allowlist,created_at FROM profiles WHERE id=?`, id).Scan(&p.ID, &p.Name, &ins, &tools, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	if err != nil {
		return p, err
	}
	_ = json.Unmarshal([]byte(ins), &p.InstallationIDs)
	_ = json.Unmarshal([]byte(tools), &p.ToolAllowlist)
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return p, nil
}

func (r *SQLiteRepository) ListProfiles(ctx context.Context) ([]domain.Profile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,installation_ids,tool_allowlist,created_at FROM profiles ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Profile
	for rows.Next() {
		var p domain.Profile
		var ins, tools, created string
		if err := rows.Scan(&p.ID, &p.Name, &ins, &tools, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ins), &p.InstallationIDs)
		_ = json.Unmarshal([]byte(tools), &p.ToolAllowlist)
		p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) SetProfileInstallations(ctx context.Context, id string, installationIDs []string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE profiles SET installation_ids=? WHERE id=?`, mustJSON(installationIDs), id)
	return err
}

func (r *SQLiteRepository) SetProfileTools(ctx context.Context, id string, tools []string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE profiles SET tool_allowlist=? WHERE id=?`, mustJSON(tools), id)
	return err
}

func (r *SQLiteRepository) CreateClient(ctx context.Context, c domain.Client, tokenHash string) (domain.Client, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO clients (id,profile_id,name,token_hash,revoked,created_at) VALUES (?,?,?,?,?,?)`,
		c.ID, c.ProfileID, c.Name, tokenHash, boolInt(c.Revoked), c.CreatedAt.UTC().Format(time.RFC3339Nano))
	return c, err
}

func (r *SQLiteRepository) GetClientAuth(ctx context.Context, id string) (client domain.Client, tokenHash string, ok bool, err error) {
	var created string
	var revoked int
	err = r.db.QueryRowContext(ctx, `SELECT id,profile_id,name,revoked,created_at,token_hash FROM clients WHERE id=?`, id).
		Scan(&client.ID, &client.ProfileID, &client.Name, &revoked, &created, &tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return client, "", false, nil
	}
	if err != nil {
		return client, "", false, err
	}
	client.Revoked = revoked == 1
	client.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return client, tokenHash, true, nil
}

func (r *SQLiteRepository) ListClients(ctx context.Context) ([]domain.Client, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,profile_id,name,revoked,created_at FROM clients ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Client
	for rows.Next() {
		var c domain.Client
		var created string
		var revoked int
		if err := rows.Scan(&c.ID, &c.ProfileID, &c.Name, &revoked, &created); err != nil {
			return nil, err
		}
		c.Revoked = revoked == 1
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) RevokeClient(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET revoked=1 WHERE id=?`, id)
	return err
}

func (r *SQLiteRepository) CreateSecretReference(ctx context.Context, ref domain.SecretReference, opaqueValue string) (domain.SecretReference, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO secret_references (id,name,description,secret_cipher,created_at) VALUES (?,?,?,?,?)`,
		ref.ID, ref.Name, ref.Description, opaqueValue, ref.CreatedAt.UTC().Format(time.RFC3339Nano))
	return ref, err
}

func (r *SQLiteRepository) ListSecretReferences(ctx context.Context) ([]domain.SecretReference, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,description,created_at FROM secret_references ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.SecretReference
	for rows.Next() {
		var ref domain.SecretReference
		var created string
		if err := rows.Scan(&ref.ID, &ref.Name, &ref.Description, &created); err != nil {
			return nil, err
		}
		ref.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, ref)
	}
	return out, rows.Err()
}

func (r *SQLiteRepository) CreateAuditEvent(ctx context.Context, e domain.AuditEvent) (domain.AuditEvent, error) {
	_, err := r.db.ExecContext(ctx, `INSERT INTO audit_events (id,actor,action,target,result,metadata,redacted,request_id,remote_addr,profile_id,client_id,occurred_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.Actor, e.Action, e.Target, e.Result, mustJSON(e.Metadata), boolInt(e.Redacted), e.RequestID, e.RemoteAddr, e.ProfileID, e.ClientID, e.OccurredAt.UTC().Format(time.RFC3339Nano))
	return e, err
}

func (r *SQLiteRepository) ListAuditEvents(ctx context.Context, limit int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,actor,action,target,result,metadata,redacted,request_id,remote_addr,profile_id,client_id,occurred_at FROM audit_events ORDER BY occurred_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var metadata, occurred string
		var redacted int
		if err := rows.Scan(&e.ID, &e.Actor, &e.Action, &e.Target, &e.Result, &metadata, &redacted, &e.RequestID, &e.RemoteAddr, &e.ProfileID, &e.ClientID, &occurred); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(metadata), &e.Metadata)
		e.Redacted = redacted == 1
		e.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		out = append(out, e)
	}
	return out, rows.Err()
}

func mustJSON(v any) string {
	if v == nil {
		return "{}"
	}
	switch vv := v.(type) {
	case []string:
		if len(vv) == 0 {
			return "[]"
		}
	case []domain.ToolMetadata:
		if len(vv) == 0 {
			return "[]"
		}
	case []domain.PolicyFinding:
		if len(vv) == 0 {
			return "[]"
		}
	case []domain.DeploymentPlan:
		if len(vv) == 0 {
			return "[]"
		}
	case map[string]string:
		if len(vv) == 0 {
			return "{}"
		}
	case map[string]any:
		if len(vv) == 0 {
			return "{}"
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339Nano)
}
