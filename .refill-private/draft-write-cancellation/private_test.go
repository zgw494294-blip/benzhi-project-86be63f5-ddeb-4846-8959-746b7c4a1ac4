package draftwritecancellation_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestCanceledDraftWritesDoNotCommit(t *testing.T) {
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store).WithClock(func() time.Time { return now })
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	violations := make([]string, 0, 6)
	requireCanceled := func(operation string, err error) {
		t.Helper()
		if !errors.Is(err, context.Canceled) {
			violations = append(violations, fmt.Sprintf("%s 返回 %v", operation, err))
		}
	}

	_, err = service.CreateCase(canceled, "cancel-create", application.CreateCaseCommand{
		Title: "不应创建", IntervieweeRef: "I-canceled", IntendedUse: "研究", Actor: "organizer:甲",
	})
	requireCanceled("CreateCase", err)

	ctx := context.Background()
	view, err := service.CreateCase(ctx, "create-live", application.CreateCaseCommand{
		Title: "有效案卷", IntervieweeRef: "I-live", IntendedUse: "研究", Actor: "organizer:甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	caseID := view.Case.ID
	currentVersion := func() int64 {
		t.Helper()
		current, getErr := service.GetCase(ctx, caseID)
		if getErr != nil {
			t.Fatal(getErr)
		}
		return current.Case.Version
	}

	_, err = service.UpdateMetadata(canceled, caseID, "cancel-metadata", application.MetadataCommand{
		Title: "被取消的标题", IntervieweeRef: "I-live", IntendedUse: "研究", Actor: "organizer:甲", ExpectedVersion: currentVersion(),
	})
	requireCanceled("UpdateMetadata", err)

	_, err = service.AddSegment(canceled, caseID, "cancel-add", application.SegmentCommand{
		ID: "canceled-single", StartMillis: 0, EndMillis: 10, SourceText: "不应写入", ProposedText: "不应写入", SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: currentVersion(),
	})
	requireCanceled("AddSegment", err)

	_, err = service.AddSegmentsBatch(canceled, caseID, "cancel-batch", application.BatchSegmentsCommand{
		Actor: "organizer:甲", ExpectedVersion: currentVersion(),
		Segments: []application.BatchSegmentItem{{ID: "canceled-batch", StartMillis: 10, EndMillis: 20, SourceText: "不应批量写入", ProposedText: "不应批量写入", SensitivityTag: domain.SensitivityNone}},
	})
	requireCanceled("AddSegmentsBatch", err)

	view, err = service.AddSegment(ctx, caseID, "add-live", application.SegmentCommand{
		ID: "live", StartMillis: 20, EndMillis: 30, SourceText: "保留内容", ProposedText: "保留内容", SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: currentVersion(),
	})
	if err != nil {
		t.Fatal(err)
	}
	segmentVersion := view.Case.Version

	_, err = service.UpdateSegment(canceled, caseID, "live", "cancel-update", application.SegmentCommand{
		StartMillis: 20, EndMillis: 30, SourceText: "被取消的修改", ProposedText: "被取消的修改", SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: currentVersion(),
	})
	requireCanceled("UpdateSegment", err)

	_, err = service.DeleteSegment(canceled, caseID, "live", "cancel-delete", application.VersionCommand{
		Actor: "organizer:甲", ExpectedVersion: currentVersion(),
	})
	requireCanceled("DeleteSegment", err)

	got, err := service.GetCase(ctx, caseID)
	if err != nil {
		t.Fatal(err)
	}
	cases, err := service.ListCases(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 0 || len(cases) != 1 || got.Case.Version != segmentVersion || got.Case.Title != "有效案卷" || len(got.Case.Segments) != 1 || got.Case.Segments[0].ID != "live" || got.Case.Segments[0].SourceText != "保留内容" {
		t.Fatalf("已取消的草稿写入仍被提交: violations=%v cases=%d case=%#v", violations, len(cases), got.Case)
	}
}
