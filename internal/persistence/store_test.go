package persistence

import (
	"context"
	"errors"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
)

func TestStorePersistsIdempotencyAndAuditAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, _ := domain.NewReleaseCase("case-1", "标题", "I", "教育", "organizer:甲", time.Now())
	meta := application.MutationMeta{Key: "key-1", Fingerprint: "fp-1", Actor: "organizer:甲", Action: "create"}
	first, replay, err := store.Create(ctx, c, meta)
	if err != nil || replay {
		t.Fatalf("首次提交: replay=%v err=%v", replay, err)
	}
	store2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, replay, err := store2.Create(ctx, c, meta)
	if err != nil || !replay || second.Version != first.Version {
		t.Fatalf("恢复后幂等重放失败: replay=%v err=%v", replay, err)
	}
	_, _, err = store2.Create(ctx, c, application.MutationMeta{Key: "key-1", Fingerprint: "different", Actor: "organizer:甲", Action: "create"})
	if !errors.Is(err, domain.ErrIdempotency) {
		t.Fatalf("冲突重用得到 %v", err)
	}
}

func TestStoreComparesExpectedVersion(t *testing.T) {
	store, _ := Open(t.TempDir())
	c, _ := domain.NewReleaseCase("case-1", "标题", "I", "教育", "organizer:甲", time.Now())
	ctx := context.Background()
	_, _, _ = store.Create(ctx, c, application.MutationMeta{Key: "create", Fingerprint: "a", Actor: "organizer:甲", Action: "create"})
	_, _, err := store.Update(ctx, c.ID, 99, application.MutationMeta{Key: "update", Fingerprint: "b"}, func(*domain.ReleaseCase) error { return nil })
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
}
