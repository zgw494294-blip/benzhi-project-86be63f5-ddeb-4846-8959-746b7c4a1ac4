package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTimelineAndFrozenOriginalAreProtected(t *testing.T) {
	now := time.Date(2026, 8, 26, 1, 0, 0, 0, time.UTC)
	c, err := NewReleaseCase("c1", "村落记忆", "INT-1", "公共教育", "organizer:甲", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddSegment(TranscriptSegment{ID: "s1", StartMillis: 100, EndMillis: 300, SourceText: "第一段"}, "organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	if err := c.AddSegment(TranscriptSegment{ID: "s2", StartMillis: 200, EndMillis: 400, SourceText: "重叠段"}, "organizer:甲", now); err == nil {
		t.Fatal("应拒绝重叠片段")
	}
	grant := ConsentGrant{ID: "g1", Scope: []string{"*"}, AllowedUses: []string{"公共教育"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "受访者", SignedAt: now}
	if err := c.AddConsent(grant, "organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Freeze("organizer:甲", now); err != nil {
		t.Fatal(err)
	}
	err = c.UpdateSegment(TranscriptSegment{ID: "s1", StartMillis: 100, EndMillis: 300, SourceText: "被篡改"}, "organizer:甲", now)
	if !errors.Is(err, ErrInvalidState) {
		t.Fatalf("冻结后更新得到 %v", err)
	}
}

func TestApprovalRequiresFullCheckAndCannotRepeat(t *testing.T) {
	now := time.Now().UTC()
	c, _ := NewReleaseCase("c1", "标题", "I", "教育", "organizer:甲", now)
	_ = c.AddSegment(TranscriptSegment{ID: "s", StartMillis: 0, EndMillis: 1, SourceText: "内容"}, "organizer:甲", now)
	_ = c.AddConsent(ConsentGrant{ID: "g", Scope: []string{"*"}, AllowedUses: []string{"教育"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "本人", SignedAt: now}, "organizer:甲", now)
	_ = c.Freeze("organizer:甲", now)
	cred := ReleaseCredential{ID: "cred", CaseID: c.ID, ManifestHash: "m", ConsentSnapshotHash: "c", ApprovedBy: "reviewer:乙", ApprovedAt: now, PublicSegmentIDs: []string{"s"}}
	if err := c.Approve(cred, "reviewer:乙", now); err == nil {
		t.Fatal("未全量检查不应批准")
	}
	if err := c.SetFindings(nil, nil, "reviewer:乙", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Approve(cred, "reviewer:乙", now); err != nil {
		t.Fatal(err)
	}
	if err := c.Approve(cred, "reviewer:乙", now); !errors.Is(err, ErrAlreadyApproved) {
		t.Fatalf("重复批准得到 %v", err)
	}
}
