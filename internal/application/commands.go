package application

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"oral-history-release-studio/internal/domain"
)

type CreateCaseCommand struct {
	Title          string `json:"title"`
	IntervieweeRef string `json:"intervieweeRef"`
	IntendedUse    string `json:"intendedUse"`
	Actor          string `json:"actor"`
}
type MetadataCommand struct {
	Title           string `json:"title"`
	IntervieweeRef  string `json:"intervieweeRef"`
	IntendedUse     string `json:"intendedUse"`
	Actor           string `json:"actor"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
type SegmentCommand struct {
	ID              string                `json:"id"`
	StartMillis     int64                 `json:"startMillis"`
	EndMillis       int64                 `json:"endMillis"`
	SourceText      string                `json:"sourceText"`
	SensitivityTag  domain.SensitivityTag `json:"sensitivityTag"`
	ProposedText    string                `json:"proposedText"`
	Actor           string                `json:"actor"`
	ExpectedVersion int64                 `json:"expectedVersion"`
}
type BatchSegmentItem struct {
	ID             string                `json:"id"`
	StartMillis    int64                 `json:"startMillis"`
	EndMillis      int64                 `json:"endMillis"`
	SourceText     string                `json:"sourceText"`
	SensitivityTag domain.SensitivityTag `json:"sensitivityTag"`
	ProposedText   string                `json:"proposedText"`
}
type BatchSegmentsCommand struct {
	Segments        []BatchSegmentItem `json:"segments"`
	Actor           string             `json:"actor"`
	ExpectedVersion int64              `json:"expectedVersion"`
}
type ConsentCommand struct {
	ID              string    `json:"id"`
	Scope           []string  `json:"scope"`
	AllowedUses     []string  `json:"allowedUses"`
	Restrictions    []string  `json:"restrictions"`
	ValidFrom       time.Time `json:"validFrom"`
	ExpiresAt       time.Time `json:"expiresAt"`
	SignedBy        string    `json:"signedBy"`
	SignedAt        time.Time `json:"signedAt"`
	Actor           string    `json:"actor"`
	ExpectedVersion int64     `json:"expectedVersion"`
}
type VersionCommand struct {
	Actor             string `json:"actor"`
	ExpectedVersion   int64  `json:"expectedVersion"`
	ConfirmationToken string `json:"confirmationToken,omitempty"`
}
type ReturnCommand struct {
	Actor           string `json:"actor"`
	Note            string `json:"note"`
	ExpectedVersion int64  `json:"expectedVersion"`
}
type BatchReturnItem struct {
	FindingID string `json:"findingId"`
	Note      string `json:"note"`
}
type BatchReturnCommand struct {
	Items           []BatchReturnItem `json:"items"`
	Actor           string            `json:"actor"`
	ExpectedVersion int64             `json:"expectedVersion"`
}
type RemediationCommand struct {
	Actor           string `json:"actor"`
	ProposedText    string `json:"proposedText"`
	Explanation     string `json:"explanation"`
	ExpectedVersion int64  `json:"expectedVersion"`
}

func fingerprint(action string, v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(append([]byte(action+":"), b...))
	return hex.EncodeToString(sum[:])
}
