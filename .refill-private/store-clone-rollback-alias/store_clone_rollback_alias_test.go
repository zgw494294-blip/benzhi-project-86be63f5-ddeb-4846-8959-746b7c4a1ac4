package store_clone_rollback_alias_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestFailedUpdateDoesNotAliasStoredCase(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	candidate, err := domain.NewReleaseCase("case-alias", "别名回滚案卷", "I-1", "教育", "organizer:甲", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := candidate.AddSegment(domain.TranscriptSegment{ID: "segment-1", StartMillis: 0, EndMillis: 10, SourceText: "原始文本", ProposedText: "原始文本", SensitivityTag: domain.SensitivityNone}, "organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	if err := candidate.AddConsent(domain.ConsentGrant{ID: "consent-1", Scope: []string{"*"}, AllowedUses: []string{"教育"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "本人", SignedAt: now}, "organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	if err := candidate.Freeze("organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	finding := domain.ReviewFinding{ID: "finding-1", CaseID: candidate.ID, SegmentID: "segment-1", RuleCode: "SENSITIVE_UNREMEDIATED", Severity: domain.SeverityBlocker, Status: domain.FindingOpen, CreatedAt: now}
	if err := candidate.SetFindings([]domain.ReviewFinding{finding}, nil, "reviewer:乙", now); err != nil {
		t.Fatal(err)
	}
	if err := candidate.ReturnFinding(finding.ID, "请改写原文", "reviewer:乙", now); err != nil {
		t.Fatal(err)
	}
	created, _, err := store.Create(context.Background(), candidate, application.MutationMeta{Key: "create-alias", Fingerprint: "create-alias", Actor: "organizer:甲", Action: "create_case"})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	service := application.NewService(store).WithClock(func() time.Time {
		cancel()
		return now
	})
	_, err = service.Remediate(ctx, created.ID, "segment-1", "remediate-alias", application.RemediationCommand{Actor: "organizer:甲", ExpectedVersion: created.Version, ProposedText: "概括后的公开文本", Explanation: "移除敏感细节"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("预期整改在取消后回滚，实际错误为 %v", err)
	}

	got, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	disk, err := reopened.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	memoryChanged := got.Version != created.Version || got.Segments[0].ProposedText != "原始文本" || got.Segments[0].Disposition != "" || got.Findings[0].Remediation != "" || len(got.Revisions) != 0
	diskChanged := disk.Version != created.Version || disk.Segments[0].ProposedText != "原始文本" || disk.Segments[0].Disposition != "" || disk.Findings[0].Remediation != "" || len(disk.Revisions) != 0
	if memoryChanged || diskChanged {
		t.Fatalf("取消的整改产生内存/磁盘撕裂: memory={version:%d proposedText:%q disposition:%q remediation:%q revisions:%d} disk={version:%d proposedText:%q disposition:%q remediation:%q revisions:%d}", got.Version, got.Segments[0].ProposedText, got.Segments[0].Disposition, got.Findings[0].Remediation, len(got.Revisions), disk.Version, disk.Segments[0].ProposedText, disk.Segments[0].Disposition, disk.Findings[0].Remediation, len(disk.Revisions))
	}
}
