package failed_idempotency_reservation_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestRejectedMutationIsNotReplayedAsSuccess(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 8, 0, 0, 0, time.UTC)
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store).WithClock(func() time.Time { return now })
	view, err := service.CreateCase(ctx, "create", application.CreateCaseCommand{Title: "重放案卷", IntervieweeRef: "I-1", IntendedUse: "教育", Actor: "organizer:甲"})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddSegment(ctx, view.Case.ID, "segment", application.SegmentCommand{ID: "s-1", StartMillis: 0, EndMillis: 1000, SourceText: "公开内容", ProposedText: "公开内容", SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.AddConsent(ctx, view.Case.ID, "consent", application.ConsentCommand{ID: "g-1", Scope: []string{"*"}, AllowedUses: []string{"教育"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "受访者", SignedAt: now, Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Freeze(ctx, view.Case.ID, "freeze", application.VersionCommand{Actor: "organizer:甲", ExpectedVersion: view.Case.Version})
	if err != nil {
		t.Fatal(err)
	}

	request := application.MetadataCommand{Title: "冻结后改名", IntervieweeRef: "I-1", IntendedUse: "教育", Actor: "organizer:甲", ExpectedVersion: view.Case.Version}
	if _, err := service.UpdateMetadata(ctx, view.Case.ID, "rejected-metadata", request); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("首次冻结后变更应返回 ErrInvalidState，实际为 %v", err)
	}

	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	restartedService := application.NewService(reopened).WithClock(func() time.Time { return now })
	if _, err := restartedService.UpdateMetadata(ctx, view.Case.ID, "rejected-metadata", request); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("相同失败请求重放必须再次返回 ErrInvalidState，实际为 %v", err)
	}
}
