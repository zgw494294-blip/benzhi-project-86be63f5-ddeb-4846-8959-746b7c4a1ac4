package consentcoverageexpirycache_test

import (
	"context"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestConsentCoverageCacheExpiresWithoutVersionChange(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储失败：%v", err)
	}
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	service := application.NewService(store).WithClock(func() time.Time { return now })
	ctx := context.Background()

	created, err := service.CreateCase(ctx, "expiry-create", application.CreateCaseCommand{
		Title: "授权到期缓存复现", IntervieweeRef: "interviewee-1", IntendedUse: "公开展览", Actor: "organizer:tester",
	})
	if err != nil {
		t.Fatalf("创建案卷失败：%v", err)
	}
	caseID := created.Case.ID
	withSegment, err := service.AddSegment(ctx, caseID, "expiry-segment", application.SegmentCommand{
		ID: "segment-1", StartMillis: 0, EndMillis: 1_000, SourceText: "公开片段", ProposedText: "公开片段",
		SensitivityTag: domain.SensitivityNone, Actor: "organizer:tester", ExpectedVersion: created.Case.Version,
	})
	if err != nil {
		t.Fatalf("新增片段失败：%v", err)
	}
	withConsent, err := service.AddConsent(ctx, caseID, "expiry-consent", application.ConsentCommand{
		ID: "consent-1", Scope: []string{"segment-1"}, AllowedUses: []string{"公开展览"},
		ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(30 * time.Minute), SignedBy: "受访者一",
		SignedAt: now.Add(-time.Hour), Actor: "organizer:tester", ExpectedVersion: withSegment.Case.Version,
	})
	if err != nil {
		t.Fatalf("新增授权失败：%v", err)
	}
	if !withConsent.ConsentCoverage.CanFreeze || withConsent.ConsentCoverage.Segments[0].Status != domain.CoverageCovered {
		t.Fatalf("前置条件不成立，授权应在初始时刻覆盖片段：%+v", withConsent.ConsentCoverage)
	}

	versionBeforeExpiry := withConsent.Case.Version
	now = now.Add(time.Hour)
	afterExpiry, err := service.GetCase(ctx, caseID)
	if err != nil {
		t.Fatalf("授权到期后读取案卷失败：%v", err)
	}
	if afterExpiry.Case.Version != versionBeforeExpiry {
		t.Fatalf("前置条件不成立，推进时钟不应修改 version：得到 %d，期望 %d", afterExpiry.Case.Version, versionBeforeExpiry)
	}
	if len(afterExpiry.ConsentCoverage.Segments) != 1 {
		t.Fatalf("应返回一个片段覆盖结果，实际为：%+v", afterExpiry.ConsentCoverage)
	}
	if afterExpiry.ConsentCoverage.CanFreeze || afterExpiry.ConsentCoverage.Segments[0].Status != domain.CoverageExpired {
		t.Fatalf("已到期授权仍从同一 version 的缓存中复用：%+v", afterExpiry.ConsentCoverage)
	}
}
