package idempotency_result_cache_alias_test

import (
	"context"
	"testing"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/persistence"
)

func TestIdempotencyReplayResultCannotPoisonCache(t *testing.T) {
	ctx := context.Background()
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开存储失败: %v", err)
	}
	service := application.NewService(store)
	created, err := service.CreateCase(ctx, "create-cache-alias", application.CreateCaseCommand{
		Title:          "原始标题",
		IntervieweeRef: "interviewee-1",
		IntendedUse:    "公开展览",
		Actor:          "organizer:alice",
	})
	if err != nil {
		t.Fatalf("创建案卷失败: %v", err)
	}
	cmd := application.MetadataCommand{
		Title:           "可信标题",
		IntervieweeRef:  "interviewee-1",
		IntendedUse:     "公开展览",
		Actor:           "organizer:alice",
		ExpectedVersion: created.Case.Version,
	}
	if _, err := service.UpdateMetadata(ctx, created.Case.ID, "metadata-cache-alias", cmd); err != nil {
		t.Fatalf("首次更新失败: %v", err)
	}
	replayed, err := service.UpdateMetadata(ctx, created.Case.ID, "metadata-cache-alias", cmd)
	if err != nil {
		t.Fatalf("首次重放失败: %v", err)
	}
	replayed.Case.Title = "调用方污染标题"
	replayed.Case.Events[0].Details = map[string]any{"poisoned": true}

	again, err := service.UpdateMetadata(ctx, created.Case.ID, "metadata-cache-alias", cmd)
	if err != nil {
		t.Fatalf("再次重放失败: %v", err)
	}
	if again.Case.Title != "可信标题" {
		t.Fatalf("幂等结果缓存被调用方污染: got title %q", again.Case.Title)
	}
	if _, poisoned := again.Case.Events[0].Details["poisoned"]; poisoned {
		t.Fatal("幂等结果缓存的嵌套事件 map 被调用方污染")
	}

	stored, err := service.GetCase(ctx, created.Case.ID)
	if err != nil {
		t.Fatalf("读取持久化案卷失败: %v", err)
	}
	if stored.Case.Title != "可信标题" {
		t.Fatalf("持久化案卷不应被污染: got title %q", stored.Case.Title)
	}
}
