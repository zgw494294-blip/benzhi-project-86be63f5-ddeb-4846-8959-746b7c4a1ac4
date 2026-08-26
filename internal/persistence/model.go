package persistence

import (
	"time"

	"oral-history-release-studio/internal/domain"
)

const schemaVersion = 1

type snapshot struct {
	SchemaVersion         int                                    `json:"schemaVersion"`
	Cases                 map[string]*domain.ReleaseCase         `json:"cases"`
	Idempotency           map[string]idempotencyRecord           `json:"idempotency"`
	Audit                 []auditEntry                           `json:"audit"`
	ApprovalConfirmations map[string]domain.ApprovalConfirmation `json:"approvalConfirmations"`
}

type idempotencyRecord struct {
	Fingerprint string              `json:"fingerprint"`
	CaseID      string              `json:"caseId"`
	Result      *domain.ReleaseCase `json:"result"`
}

type auditEntry struct {
	Sequence     int64     `json:"sequence"`
	CaseID       string    `json:"caseId"`
	CaseVersion  int64     `json:"caseVersion"`
	Action       string    `json:"action"`
	Actor        string    `json:"actor"`
	At           time.Time `json:"at"`
	PreviousHash string    `json:"previousHash"`
	Hash         string    `json:"hash"`
}
