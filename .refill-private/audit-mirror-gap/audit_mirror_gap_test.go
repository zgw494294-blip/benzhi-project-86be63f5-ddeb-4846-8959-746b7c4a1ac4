package auditmirrorgap_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
	"oral-history-release-studio/internal/persistence"
)

func TestAuditMirrorFailureDoesNotPoisonNextCommit(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	auditPath := filepath.Join(dir, "audit.jsonl")
	if err := os.Mkdir(auditPath, 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 13, 0, 0, 0, time.UTC)
	candidate, err := domain.NewReleaseCase("case-1", "审计案卷", "I", "教育", "organizer:甲", now)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Create(context.Background(), candidate, application.MutationMeta{Key: "create", Fingerprint: "create-fp", Actor: "organizer:甲", Action: "create_case"})
	if err == nil {
		t.Fatal("审计镜像路径为目录时首次提交应报告镜像写入失败")
	}
	if err := os.Remove(auditPath); err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Update(context.Background(), candidate.ID, 1, application.MutationMeta{Key: "update", Fingerprint: "update-fp", Actor: "organizer:甲", Action: "update_metadata"}, func(c *domain.ReleaseCase) error {
		return c.UpdateMetadata("审计案卷二", "I", "教育", "organizer:甲", now.Add(time.Minute))
	})
	if err != nil {
		t.Fatalf("镜像路径恢复后下一次提交失败: %v", err)
	}
	if _, err := persistence.Open(dir); err != nil {
		t.Fatalf("后续成功提交后存储无法重新打开: %v", err)
	}
}
