package generatedididempotency_test

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestGeneratedIDRequestReplaysWithSameIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC)
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store).WithClock(func() time.Time { return now })
	view, err := service.CreateCase(ctx, "create", application.CreateCaseCommand{
		Title: "幂等案卷", IntervieweeRef: "I", IntendedUse: "教育", Actor: "organizer:甲",
	})
	if err != nil {
		t.Fatal(err)
	}
	originalReader := cryptorand.Reader
	cryptorand.Reader = bytes.NewReader(append(bytes.Repeat([]byte{1}, 12), bytes.Repeat([]byte{2}, 12)...))
	defer func() { cryptorand.Reader = originalReader }()
	cmd := application.SegmentCommand{
		StartMillis: 0, EndMillis: 10, SourceText: "内容", ProposedText: "内容",
		SensitivityTag: domain.SensitivityNone, Actor: "organizer:甲", ExpectedVersion: view.Case.Version,
	}
	first, err := service.AddSegment(ctx, view.Case.ID, "same-segment-request", cmd)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.AddSegment(ctx, view.Case.ID, "same-segment-request", cmd)
	if err != nil {
		t.Fatalf("同一请求使用同一 Idempotency-Key 重放失败: %v", err)
	}
	if replayed.Case.Version != first.Case.Version || replayed.Case.Segments[0].ID != first.Case.Segments[0].ID {
		t.Fatalf("重放没有返回首次提交结果: first=%#v replayed=%#v", first.Case, replayed.Case)
	}
}
