package finding_query_stale_version_test

import (
	"context"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestFindingQueryDoesNotReusePriorCaseVersion(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store).WithClock(func() time.Time { return now })

	view, err := service.CreateCase(ctx, "create", application.CreateCaseCommand{
		Title: "待整改案卷", IntervieweeRef: "I-1", IntendedUse: "展览", Actor: "organizer:甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := view.Case.ID
	view, err = service.AddSegment(ctx, caseID, "segment", application.SegmentCommand{
		ID: "sensitive-1", StartMillis: 0, EndMillis: 1000, SourceText: "未经处理的家庭经历",
		ProposedText: "未经处理的家庭经历", SensitivityTag: domain.SensitivityPersonal,
		Actor: "organizer:甲", ExpectedVersion: view.Case.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddConsent(ctx, caseID, "consent", application.ConsentCommand{
		ID: "consent-1", Scope: []string{"*"}, AllowedUses: []string{"展览"},
		ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "受访者", SignedAt: now,
		Actor: "organizer:甲", ExpectedVersion: view.Case.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Freeze(ctx, caseID, "freeze", application.VersionCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.RunChecks(ctx, caseID, "full-check", application.VersionCommand{Actor: "reviewer:乙", ExpectedVersion: view.Case.Version}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Case.Findings) != 1 || view.Case.Findings[0].Status != domain.FindingOpen {
		t.Fatalf("未建立预期的开放发现: %#v", view.Case.Findings)
	}

	first, err := service.QueryFindings(ctx, caseID, application.FindingQuery{})
	if err != nil {
		t.Fatal(err)
	}
	findingID := first.Findings[0].ID
	view, err = service.ReturnFinding(ctx, caseID, findingID, "return", application.ReturnCommand{
		Actor: "reviewer:乙", Note: "请移除可识别家庭细节", ExpectedVersion: view.Case.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Remediate(ctx, caseID, "sensitive-1", "remediate", application.RemediationCommand{
		Actor: "organizer:甲", ProposedText: "一段已概括的成长经历", Explanation: "移除家庭识别细节", ExpectedVersion: view.Case.Version,
	})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.RunChecks(ctx, caseID, "targeted-check", application.VersionCommand{
		Actor: "reviewer:乙", ExpectedVersion: view.Case.Version,
	}, []string{"sensitive-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Case.Findings) != 1 || view.Case.Findings[0].Status != domain.FindingClosed {
		t.Fatalf("定向复检未关闭发现: %#v", view.Case.Findings)
	}

	after, err := service.QueryFindings(ctx, caseID, application.FindingQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if after.CaseVersion != view.Case.Version || len(after.Findings) != 1 || after.Findings[0].Status != domain.FindingClosed {
		t.Fatalf("查询复用了旧案卷版本: got version=%d status=%s, want version=%d status=%s", after.CaseVersion, after.Findings[0].Status, view.Case.Version, domain.FindingClosed)
	}
}
