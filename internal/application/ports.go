package application

import (
	"context"
	"time"

	"oral-history-release-studio/internal/domain"
)

type MutationMeta struct {
	Key         string
	Fingerprint string
	Actor       string
	Action      string
}

type Repository interface {
	Create(context.Context, *domain.ReleaseCase, MutationMeta) (*domain.ReleaseCase, bool, error)
	Get(context.Context, string) (*domain.ReleaseCase, error)
	List(context.Context) ([]*domain.ReleaseCase, error)
	Update(context.Context, string, int64, MutationMeta, func(*domain.ReleaseCase) error) (*domain.ReleaseCase, bool, error)
	SaveApprovalConfirmation(context.Context, domain.ApprovalConfirmation) error
	UpdateWithApprovalConfirmation(context.Context, string, int64, MutationMeta, domain.ApprovalConfirmation, time.Time, func(*domain.ReleaseCase) error) (*domain.ReleaseCase, bool, error)
}
