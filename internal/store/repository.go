package store

import (
	"context"

	"github.com/mohvahedi/open-mcp-control-plane/internal/domain"
)

type Repository interface {
	Ping(context.Context) error

	GetAdminHash(context.Context) (string, error)
	SetAdminHash(context.Context, string) error

	CreateDeploymentPlan(context.Context, domain.DeploymentPlan) (domain.DeploymentPlan, error)
	GetDeploymentPlan(context.Context, string) (domain.DeploymentPlan, error)
	ListDeploymentPlans(context.Context) ([]domain.DeploymentPlan, error)

	CreateApproval(context.Context, domain.Approval) (domain.Approval, error)
	GetApprovalByPlanID(context.Context, string) (domain.Approval, bool, error)
	ListApprovals(context.Context) ([]domain.Approval, error)

	CreateInstallation(context.Context, domain.Installation) (domain.Installation, error)
	GetInstallation(context.Context, string) (domain.Installation, error)
	ListInstallations(context.Context) ([]domain.Installation, error)
	UpdateInstallation(context.Context, domain.Installation) error
	DisableInstallation(context.Context, string) error
	DeleteInstallation(context.Context, string) error

	CreateProfile(context.Context, domain.Profile) (domain.Profile, error)
	GetProfile(context.Context, string) (domain.Profile, error)
	ListProfiles(context.Context) ([]domain.Profile, error)
	SetProfileInstallations(context.Context, string, []string) error
	SetProfileTools(context.Context, string, []string) error

	CreateClient(context.Context, domain.Client, string) (domain.Client, error)
	GetClientAuth(context.Context, string) (client domain.Client, tokenHash string, ok bool, err error)
	ListClients(context.Context) ([]domain.Client, error)
	RevokeClient(context.Context, string) error

	CreateSecretReference(context.Context, domain.SecretReference, string) (domain.SecretReference, error)
	ListSecretReferences(context.Context) ([]domain.SecretReference, error)

	CreateAuditEvent(context.Context, domain.AuditEvent) (domain.AuditEvent, error)
	ListAuditEvents(context.Context, int) ([]domain.AuditEvent, error)
}
