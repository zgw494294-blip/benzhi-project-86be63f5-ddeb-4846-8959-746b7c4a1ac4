package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"oral-history-release-studio/internal/assurance"
	"oral-history-release-studio/internal/domain"
)

type ApprovalPreviewItem struct {
	SegmentID         string `json:"segmentId"`
	Sequence          int    `json:"sequence"`
	ProposedDigest    string `json:"proposedDigest"`
	DispositionDigest string `json:"dispositionDigest"`
}

type ApprovalPreview struct {
	CaseID              string                `json:"caseId"`
	ExpectedVersion     int64                 `json:"expectedVersion"`
	ApprovedVersion     int64                 `json:"approvedVersion"`
	Actor               string                `json:"actor"`
	OpenBlockers        int                   `json:"openBlockers"`
	ManifestHash        string                `json:"manifestHash"`
	ConsentSnapshotHash string                `json:"consentSnapshotHash"`
	Segments            []ApprovalPreviewItem `json:"segments"`
	ConfirmationToken   string                `json:"confirmationToken"`
	ExpiresAt           time.Time             `json:"expiresAt"`
}

func (s *Service) PreviewApproval(ctx context.Context, caseID string, cmd VersionCommand) (ApprovalPreview, error) {
	if err := authorize(cmd.Actor, roleReviewer, roleReleaseManager); err != nil {
		return ApprovalPreview{}, err
	}
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return ApprovalPreview{}, err
	}
	if c.Version != cmd.ExpectedVersion {
		return ApprovalPreview{}, fmt.Errorf("%w: 当前 version=%d", domain.ErrConflict, c.Version)
	}
	if c.State == domain.StateApproved || c.Credential != nil {
		return ApprovalPreview{}, domain.ErrAlreadyApproved
	}
	if c.State != domain.StateFrozen && c.State != domain.StateRemediation {
		return ApprovalPreview{}, domain.ErrInvalidState
	}
	if !c.FullCheckCompleted {
		return ApprovalPreview{}, domain.FieldError{Field: "checks", Message: "批准预览前必须完成全量检查"}
	}
	for _, finding := range s.checker.Check(c, nil, s.now()) {
		if finding.Status == domain.FindingOpen && finding.Severity == domain.SeverityBlocker {
			return ApprovalPreview{}, domain.FieldError{Field: "findings", Message: "仍有未关闭的阻断项，不能生成批准清单"}
		}
	}
	manifestHash, _, err := assurance.HashManifest(c)
	if err != nil {
		return ApprovalPreview{}, err
	}
	consentHash, err := assurance.HashConsentSnapshot(c)
	if err != nil {
		return ApprovalPreview{}, err
	}
	token := newID("approval")
	expires := domain.UTC(s.now().Add(10 * time.Minute))
	confirmation := domain.ApprovalConfirmation{Token: token, CaseID: caseID, ExpectedVersion: c.Version, Actor: cmd.Actor, ManifestHash: manifestHash, ConsentSnapshotHash: consentHash, ExpiresAt: expires}
	if err := s.repo.SaveApprovalConfirmation(ctx, confirmation); err != nil {
		return ApprovalPreview{}, err
	}
	preview := ApprovalPreview{CaseID: caseID, ExpectedVersion: c.Version, ApprovedVersion: c.Version + 1, Actor: cmd.Actor, OpenBlockers: c.OpenBlockers(), ManifestHash: manifestHash, ConsentSnapshotHash: consentHash, ConfirmationToken: token, ExpiresAt: expires}
	for _, item := range assurance.CredentialSegments(c) {
		preview.Segments = append(preview.Segments, ApprovalPreviewItem{SegmentID: item.SegmentID, Sequence: item.Sequence, ProposedDigest: item.ProposedDigest, DispositionDigest: item.DispositionHash})
	}
	return preview, nil
}

func (s *Service) approveWithConfirmation(ctx context.Context, caseID, key string, cmd VersionCommand) (CaseView, error) {
	if strings.TrimSpace(cmd.ConfirmationToken) == "" {
		return CaseView{}, domain.FieldError{Field: "confirmationToken", Message: "批准前必须先预览清单并提交确认令牌"}
	}
	candidate, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return CaseView{}, err
	}
	manifestHash, _, err := assurance.HashManifest(candidate)
	if err != nil {
		return CaseView{}, err
	}
	consentHash, err := assurance.HashConsentSnapshot(candidate)
	if err != nil {
		return CaseView{}, err
	}
	claimed := domain.ApprovalConfirmation{Token: strings.TrimSpace(cmd.ConfirmationToken), CaseID: caseID, ExpectedVersion: cmd.ExpectedVersion, Actor: cmd.Actor, ManifestHash: manifestHash, ConsentSnapshotHash: consentHash}
	now := s.now()
	c, _, err := s.repo.UpdateWithApprovalConfirmation(ctx, caseID, cmd.ExpectedVersion, meta(key, "approve", cmd.Actor, cmd), claimed, now, func(c *domain.ReleaseCase) error {
		for _, finding := range s.checker.Check(c, nil, now) {
			if finding.Status == domain.FindingOpen && finding.Severity == domain.SeverityBlocker {
				return domain.FieldError{Field: "findings", Message: "批准时重新校验发现未关闭的阻断项"}
			}
		}
		credential := domain.ReleaseCredential{ID: newID("credential"), CaseID: c.ID, ManifestHash: manifestHash, ConsentSnapshotHash: consentHash, ApprovedBy: cmd.Actor, ApprovedAt: domain.UTC(now), PublicSegmentIDs: credentialSegmentIDs(c), Segments: assurance.CredentialSegments(c)}
		return c.Approve(credential, cmd.Actor, now)
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func credentialSegmentIDs(c *domain.ReleaseCase) []string {
	items := assurance.CredentialSegments(c)
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.SegmentID
	}
	return ids
}
