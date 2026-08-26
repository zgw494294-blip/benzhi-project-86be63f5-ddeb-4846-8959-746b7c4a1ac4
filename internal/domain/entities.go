package domain

import "time"

type TranscriptSegment struct {
	ID             string         `json:"id"`
	CaseID         string         `json:"caseId"`
	Sequence       int            `json:"sequence"`
	StartMillis    int64          `json:"startMillis"`
	EndMillis      int64          `json:"endMillis"`
	SourceText     string         `json:"sourceText"`
	SensitivityTag SensitivityTag `json:"sensitivityTag"`
	ProposedText   string         `json:"proposedText"`
	Disposition    string         `json:"disposition"`
}

type ConsentGrant struct {
	ID           string    `json:"id"`
	CaseID       string    `json:"caseId"`
	Scope        []string  `json:"scope"`
	AllowedUses  []string  `json:"allowedUses"`
	Restrictions []string  `json:"restrictions"`
	ValidFrom    time.Time `json:"validFrom"`
	ExpiresAt    time.Time `json:"expiresAt"`
	SignedBy     string    `json:"signedBy"`
	SignedAt     time.Time `json:"signedAt"`
}

type ReviewFinding struct {
	ID           string        `json:"id"`
	CaseID       string        `json:"caseId"`
	SegmentID    string        `json:"segmentId"`
	RuleCode     string        `json:"ruleCode"`
	Severity     Severity      `json:"severity"`
	Status       FindingStatus `json:"status"`
	Remediation  string        `json:"remediation,omitempty"`
	ReviewerNote string        `json:"reviewerNote,omitempty"`
	Returned     bool          `json:"returned"`
	CreatedAt    time.Time     `json:"createdAt"`
	ClosedAt     *time.Time    `json:"closedAt,omitempty"`
}

type SegmentRevision struct {
	ID                 string                     `json:"id"`
	SegmentID          string                     `json:"segmentId"`
	BeforeText         string                     `json:"beforeText"`
	AfterText          string                     `json:"afterText"`
	BeforeDigest       string                     `json:"beforeDigest"`
	AfterDigest        string                     `json:"afterDigest"`
	Explanation        string                     `json:"explanation"`
	Actor              string                     `json:"actor"`
	CreatedAt          time.Time                  `json:"createdAt"`
	SubmittedVersion   int64                      `json:"submittedVersion"`
	RelatedFindingIDs  []string                   `json:"relatedFindingIds"`
	RelatedRuleCodes   []string                   `json:"relatedRuleCodes"`
	RelatedReviews     []RevisionFindingReference `json:"relatedReviews"`
	ClosedRuleCodes    []string                   `json:"closedRuleCodes,omitempty"`
	RemainingRuleCodes []string                   `json:"remainingRuleCodes,omitempty"`
	NewRuleCodes       []string                   `json:"newRuleCodes,omitempty"`
	RecheckedAt        *time.Time                 `json:"recheckedAt,omitempty"`
}

type RevisionFindingReference struct {
	FindingID    string `json:"findingId"`
	RuleCode     string `json:"ruleCode"`
	ReviewerNote string `json:"reviewerNote"`
}

type CredentialSegment struct {
	SegmentID       string `json:"segmentId"`
	Sequence        int    `json:"sequence"`
	ProposedDigest  string `json:"proposedDigest"`
	DispositionHash string `json:"dispositionDigest"`
}

type ReleaseCredential struct {
	ID                  string              `json:"id"`
	CaseID              string              `json:"caseId"`
	ApprovedVersion     int64               `json:"approvedVersion"`
	ManifestHash        string              `json:"manifestHash"`
	ApprovedBy          string              `json:"approvedBy"`
	ApprovedAt          time.Time           `json:"approvedAt"`
	PublicSegmentIDs    []string            `json:"publicSegmentIds"`
	ConsentSnapshotHash string              `json:"consentSnapshotHash"`
	Segments            []CredentialSegment `json:"segments"`
}

type ApprovalConfirmation struct {
	Token               string     `json:"token"`
	CaseID              string     `json:"caseId"`
	ExpectedVersion     int64      `json:"expectedVersion"`
	Actor               string     `json:"actor"`
	ManifestHash        string     `json:"manifestHash"`
	ConsentSnapshotHash string     `json:"consentSnapshotHash"`
	ExpiresAt           time.Time  `json:"expiresAt"`
	UsedAt              *time.Time `json:"usedAt,omitempty"`
}
