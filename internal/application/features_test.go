package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestBatchSegmentsCoverageApprovalAndCredential(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store).WithClock(func() time.Time { return now })
	view, err := service.CreateCase(ctx, "create", application.CreateCaseCommand{Title: "案卷", IntervieweeRef: "I", IntendedUse: "教育", Actor: "organizer:甲"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddSegment(ctx, view.Case.ID, "existing", application.SegmentCommand{ID: "s2", StartMillis: 20, EndMillis: 30, SourceText: "二", ProposedText: "二", SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}
	batch := application.BatchSegmentsCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version, Segments: []application.BatchSegmentItem{
		{ID: "s4", StartMillis: 40, EndMillis: 50, SourceText: "四", ProposedText: "四", SensitivityTag: domain.SensitivityNone},
		{ID: "s1", StartMillis: 0, EndMillis: 10, SourceText: "一", ProposedText: "一", SensitivityTag: domain.SensitivityNone},
		{ID: "s3", StartMillis: 30, EndMillis: 40, SourceText: "三", ProposedText: "三", SensitivityTag: domain.SensitivityNone},
	}}
	view, err = service.AddSegmentsBatch(ctx, view.Case.ID, "batch", batch)
	if err != nil {
		t.Fatal(err)
	}
	if view.Case.Version != 3 || len(view.Case.Segments) != 4 {
		t.Fatalf("批量结果 version=%d segments=%d", view.Case.Version, len(view.Case.Segments))
	}
	for index, segment := range view.Case.Segments {
		if segment.Sequence != index+1 {
			t.Fatalf("片段未连续重排: %#v", view.Case.Segments)
		}
	}
	bad := application.BatchSegmentsCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version, Segments: []application.BatchSegmentItem{{ID: "bad", StartMillis: 25, EndMillis: 26, SourceText: "冲突", ProposedText: "冲突", SensitivityTag: domain.SensitivityNone}}}
	if _, err = service.AddSegmentsBatch(ctx, view.Case.ID, "bad", bad); err == nil {
		t.Fatal("重叠批次应失败")
	}
	unchanged, _ := service.GetCase(ctx, view.Case.ID)
	if unchanged.Case.Version != 3 || len(unchanged.Case.Segments) != 4 {
		t.Fatal("失败批次改变了案卷")
	}
	consent := application.ConsentCommand{ID: "g", Scope: []string{"*"}, AllowedUses: []string{"教育"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "本人", SignedAt: now, Actor: "organizer:甲", ExpectedVersion: unchanged.Case.Version}
	view, err = service.AddConsent(ctx, view.Case.ID, "consent", consent)
	if err != nil || !view.ConsentCoverage.CanFreeze {
		t.Fatalf("授权覆盖失败: %v %#v", err, view.ConsentCoverage)
	}
	view, err = service.Freeze(ctx, view.Case.ID, "freeze", application.VersionCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.RunChecks(ctx, view.Case.ID, "check", application.VersionCommand{Actor: "reviewer:乙", ExpectedVersion: view.Case.Version}, nil)
	if err != nil || view.OpenBlockers != 0 {
		t.Fatalf("检查失败: %v blockers=%d", err, view.OpenBlockers)
	}
	preview, err := service.PreviewApproval(ctx, view.Case.ID, application.VersionCommand{Actor: "release_manager:丙", ExpectedVersion: view.Case.Version})
	if err != nil || len(preview.Segments) != 4 {
		t.Fatalf("批准预览失败: %v %#v", err, preview)
	}
	view, err = service.Approve(ctx, view.Case.ID, "approve", application.VersionCommand{Actor: "release_manager:丙", ExpectedVersion: view.Case.Version, ConfirmationToken: preview.ConfirmationToken})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := service.VerifyCredential(ctx, view.Case.ID)
	if err != nil || verified.CredentialValid == nil || !*verified.CredentialValid || len(verified.CredentialVerification.Segments) != 4 {
		t.Fatalf("逐段凭据校验失败: %v %#v", err, verified.CredentialVerification)
	}
	_, err = service.Approve(ctx, view.Case.ID, "reuse", application.VersionCommand{Actor: "release_manager:丙", ExpectedVersion: preview.ExpectedVersion, ConfirmationToken: preview.ConfirmationToken})
	if !errors.Is(err, domain.ErrTokenUsed) {
		t.Fatalf("复用令牌错误=%v", err)
	}
}

func TestBatchReturnIsAtomicAndRevisionKeepsRuleDiff(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 9, 0, 0, 0, time.UTC)
	store, _ := persistence.Open(t.TempDir())
	service := application.NewService(store).WithClock(func() time.Time { return now })
	view, _ := service.CreateCase(ctx, "create-r", application.CreateCaseCommand{Title: "整改案卷", IntervieweeRef: "I", IntendedUse: "展览", Actor: "organizer:甲"})
	batch := application.BatchSegmentsCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version, Segments: []application.BatchSegmentItem{
		{ID: "a", StartMillis: 0, EndMillis: 10, SourceText: "家庭细节甲", ProposedText: "家庭细节甲", SensitivityTag: domain.SensitivityPersonal},
		{ID: "b", StartMillis: 10, EndMillis: 20, SourceText: "家庭细节乙", ProposedText: "家庭细节乙", SensitivityTag: domain.SensitivityPersonal},
	}}
	view, _ = service.AddSegmentsBatch(ctx, view.Case.ID, "batch-r", batch)
	view, _ = service.AddConsent(ctx, view.Case.ID, "consent-r", application.ConsentCommand{ID: "g", Scope: []string{"*"}, AllowedUses: []string{"展览"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "本人", SignedAt: now, Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	view, _ = service.Freeze(ctx, view.Case.ID, "freeze-r", application.VersionCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	view, _ = service.RunChecks(ctx, view.Case.ID, "check-r", application.VersionCommand{Actor: "reviewer:乙", ExpectedVersion: view.Case.Version}, nil)
	if len(view.Case.Findings) != 2 {
		t.Fatalf("预期两个阻断发现，实际 %d", len(view.Case.Findings))
	}
	beforeReturn := view.Case.Version
	returnCmd := application.BatchReturnCommand{Actor: "reviewer:乙", ExpectedVersion: beforeReturn, Items: []application.BatchReturnItem{{FindingID: view.Case.Findings[0].ID, Note: "改写甲"}, {FindingID: view.Case.Findings[1].ID, Note: "改写乙"}}}
	view, err := service.ReturnFindingsBatch(ctx, view.Case.ID, "return-r", returnCmd)
	if err != nil || view.Case.Version != beforeReturn+1 || view.Case.State != domain.StateRemediation {
		t.Fatalf("批量退回失败: %v version=%d", err, view.Case.Version)
	}
	firstID, firstFinding := view.Case.Findings[0].SegmentID, view.Case.Findings[0].ID
	view, err = service.Remediate(ctx, view.Case.ID, firstID, "remediate-r", application.RemediationCommand{Actor: "organizer:甲", ProposedText: "概括后的经历", Explanation: "移除家庭细节", ExpectedVersion: view.Case.Version})
	if err != nil || len(view.Case.Revisions) != 1 {
		t.Fatalf("整改修订失败: %v revisions=%d", err, len(view.Case.Revisions))
	}
	view, err = service.RunChecks(ctx, view.Case.ID, "recheck-r", application.VersionCommand{Actor: "reviewer:乙", ExpectedVersion: view.Case.Version}, []string{firstID})
	if err != nil || len(view.Case.Revisions[0].ClosedRuleCodes) != 1 || view.Case.Revisions[0].ClosedRuleCodes[0] != "SENSITIVE_UNREMEDIATED" {
		t.Fatalf("修订规则差异未保留: %v %#v", err, view.Case.Revisions[0])
	}
	versionBeforeBad := view.Case.Version
	bad := application.BatchReturnCommand{Actor: "reviewer:乙", ExpectedVersion: versionBeforeBad, Items: []application.BatchReturnItem{{FindingID: firstFinding, Note: "已关闭"}, {FindingID: view.Case.Findings[len(view.Case.Findings)-1].ID, Note: "仍开放"}}}
	if _, err = service.ReturnFindingsBatch(ctx, view.Case.ID, "bad-return-r", bad); err == nil {
		t.Fatal("混入已关闭发现的批次应整体失败")
	}
	unchanged, _ := service.GetCase(ctx, view.Case.ID)
	if unchanged.Case.Version != versionBeforeBad {
		t.Fatal("失败批量退回改变了 version")
	}
}
