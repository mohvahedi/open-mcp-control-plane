package domain

import "time"

type ToolMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Operations  []string `json:"operations,omitempty"`
}

type CatalogPackage struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version,omitempty"`
	Source      string         `json:"source"`
	Runtime     string         `json:"runtime,omitempty"`
	Transport   string         `json:"transport,omitempty"`
	License     string         `json:"license,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Tools       []ToolMetadata `json:"tools,omitempty"`
	Provenance  map[string]any `json:"provenance,omitempty"`
}

type SecretReference struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type DeploymentPlan struct {
	ID                string            `json:"id"`
	PackageID         string            `json:"package_id,omitempty"`
	Name              string            `json:"name"`
	Image             string            `json:"image,omitempty"`
	RemoteEndpoint    string            `json:"remote_endpoint,omitempty"`
	Transport         string            `json:"transport,omitempty"`
	EnvVarNames       []string          `json:"env_var_names,omitempty"`
	SecretReferences  []string          `json:"secret_references,omitempty"`
	CPULimit          string            `json:"cpu_limit,omitempty"`
	MemoryLimit       string            `json:"memory_limit,omitempty"`
	PIDLimit          int               `json:"pid_limit,omitempty"`
	HostNetwork       bool              `json:"host_network"`
	Privileged        bool              `json:"privileged"`
	HostMounts        []string          `json:"host_mounts,omitempty"`
	BroadEgress       bool              `json:"broad_egress"`
	ExposePublic      bool              `json:"expose_public"`
	RequestedTools    []ToolMetadata    `json:"requested_tools,omitempty"`
	RiskFindings      []PolicyFinding   `json:"risk_findings,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	RequiresApproval  bool              `json:"requires_approval"`
	ApprovedByID      string            `json:"approved_by_id,omitempty"`
	ApprovalCreatedAt *time.Time        `json:"approval_created_at,omitempty"`
}

type PolicyFinding struct {
	Code        string `json:"code"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Recommended string `json:"recommended,omitempty"`
}

type Approval struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type Installation struct {
	ID               string            `json:"id"`
	PlanID           string            `json:"plan_id"`
	PackageID        string            `json:"package_id,omitempty"`
	RuntimeRef       string            `json:"runtime_ref,omitempty"`
	State            string            `json:"state"`
	ImageDigest      string            `json:"image_digest,omitempty"`
	RollbackMetadata map[string]string `json:"rollback_metadata,omitempty"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
}

type Profile struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	InstallationIDs []string  `json:"installation_ids,omitempty"`
	ToolAllowlist   []string  `json:"tool_allowlist,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type Client struct {
	ID        string    `json:"id"`
	ProfileID string    `json:"profile_id"`
	Name      string    `json:"name"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditEvent struct {
	ID         string         `json:"id"`
	Actor      string         `json:"actor"`
	Action     string         `json:"action"`
	Target     string         `json:"target"`
	Result     string         `json:"result"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	OccurredAt time.Time      `json:"occurred_at"`
	Redacted   bool           `json:"redacted"`
	RequestID  string         `json:"request_id,omitempty"`
	RemoteAddr string         `json:"remote_addr,omitempty"`
	ProfileID  string         `json:"profile_id,omitempty"`
	ClientID   string         `json:"client_id,omitempty"`
}
