package multistorestalecache_test

import (
	"context"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestOverlappingStoreHandlesPreserveAllCommits(t *testing.T) {
	dir := t.TempDir()
	firstStore, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	secondStore, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 14, 0, 0, 0, time.UTC)
	firstCase, err := domain.NewReleaseCase("case-a", "案卷甲", "I-A", "教育", "organizer:甲", now)
	if err != nil {
		t.Fatal(err)
	}
	secondCase, err := domain.NewReleaseCase("case-b", "案卷乙", "I-B", "教育", "organizer:乙", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := firstStore.Create(context.Background(), firstCase, application.MutationMeta{Key: "create-a", Fingerprint: "fp-a", Actor: "organizer:甲", Action: "create_case"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := secondStore.Create(context.Background(), secondCase, application.MutationMeta{Key: "create-b", Fingerprint: "fp-b", Actor: "organizer:乙", Action: "create_case"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("两个重叠 Store 均报告提交成功后无法重新打开: %v", err)
	}
	cases, err := reopened.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Fatalf("重叠 Store 丢失已确认提交: cases=%#v", cases)
	}
}
