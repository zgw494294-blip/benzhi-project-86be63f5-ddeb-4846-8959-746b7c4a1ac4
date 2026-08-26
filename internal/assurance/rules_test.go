package assurance

import (
	"testing"
	"time"

	"oral-history-release-studio/internal/domain"
)

func TestTargetedRecheckClosesSensitiveFinding(t *testing.T) {
	now := time.Now().UTC()
	c, _ := domain.NewReleaseCase("c", "题", "I", "展览", "organizer:甲", now)
	_ = c.AddSegment(domain.TranscriptSegment{ID: "s", StartMillis: 0, EndMillis: 10, SourceText: "家庭细节", ProposedText: "家庭细节", SensitivityTag: domain.SensitivityPersonal}, "organizer:甲", now)
	_ = c.AddConsent(domain.ConsentGrant{ID: "g", Scope: []string{"s"}, AllowedUses: []string{"展览"}, ValidFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), SignedBy: "本人", SignedAt: now}, "organizer:甲", now)
	_ = c.Freeze("organizer:甲", now)
	checker := NewChecker()
	found := checker.Check(c, nil, now)
	if len(found) != 1 || found[0].RuleCode != RuleSensitiveRaw {
		t.Fatalf("发现不符预期: %#v", found)
	}
	_ = c.SetFindings(found, nil, "reviewer:乙", now)
	_ = c.RemediateSegment("s", "概括后的经历", "去除细节", "organizer:甲", now)
	next := checker.Check(c, []string{"s"}, now)
	if len(next) != 1 || next[0].Status != domain.FindingClosed {
		t.Fatalf("定向复检未关闭旧发现: %#v", next)
	}
}

func TestCredentialDetectsChangedManifest(t *testing.T) {
	c := &domain.ReleaseCase{ID: "c", State: domain.StateApproved, Version: 2, Segments: []domain.TranscriptSegment{{ID: "s", Sequence: 1, ProposedText: "公开文本"}}, Consents: []domain.ConsentGrant{}}
	mh, ids, _ := HashManifest(c)
	ch, _ := HashConsentSnapshot(c)
	c.Credential = &domain.ReleaseCredential{CaseID: "c", ApprovedVersion: 2, ManifestHash: mh, ConsentSnapshotHash: ch, PublicSegmentIDs: ids}
	if ok, _ := VerifyCredential(c); !ok {
		t.Fatal("初始凭据应有效")
	}
	c.Segments[0].ProposedText = "变化"
	if ok, _ := VerifyCredential(c); ok {
		t.Fatal("内容变化后凭据应失效")
	}
}
