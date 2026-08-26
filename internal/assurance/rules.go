package assurance

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"oral-history-release-studio/internal/domain"
)

const (
	RuleMissingConsent = "CONSENT_MISSING"
	RuleUseConflict    = "CONSENT_USE_CONFLICT"
	RuleConsentExpired = "CONSENT_EXPIRED"
	RuleSensitiveRaw   = "SENSITIVE_UNREMEDIATED"
	RuleMaskLeak       = "MASK_LEAK"
)

type Checker struct{}

func NewChecker() *Checker { return &Checker{} }

func (c *Checker) Check(rc *domain.ReleaseCase, segmentIDs []string, now time.Time) []domain.ReviewFinding {
	targets := make(map[string]bool)
	for _, id := range segmentIDs {
		targets[id] = true
	}
	full := len(targets) == 0
	open := make(map[string]domain.ReviewFinding)
	for _, s := range rc.Segments {
		if !full && !targets[s.ID] {
			continue
		}
		grants := applicableConsents(rc.Consents, s.ID)
		if len(grants) == 0 {
			addFinding(open, rc.ID, s.ID, RuleMissingConsent, "未找到覆盖该片段的授权", now)
		} else {
			valid, qualified := false, false
			for _, g := range grants {
				current := !now.Before(g.ValidFrom) && !now.After(g.ExpiresAt)
				if current {
					valid = true
				}
				useAllowed, prohibited := false, false
				for _, u := range g.AllowedUses {
					if u == "*" || strings.EqualFold(strings.TrimSpace(u), strings.TrimSpace(rc.IntendedUse)) {
						useAllowed = true
					}
				}
				for _, r := range g.Restrictions {
					if strings.Contains(r, "禁止公开") || strings.Contains(strings.ToLower(r), "no-public") {
						prohibited = true
					}
				}
				if current && useAllowed && !prohibited {
					qualified = true
				}
			}
			if !valid {
				addFinding(open, rc.ID, s.ID, RuleConsentExpired, "覆盖该片段的授权尚未生效或已经到期", now)
			}
			if valid && !qualified {
				addFinding(open, rc.ID, s.ID, RuleUseConflict, "拟公开用途超出授权边界或命中限制条件", now)
			}
		}
		if s.SensitivityTag != domain.SensitivityNone && (strings.TrimSpace(s.Disposition) == "" || strings.TrimSpace(s.ProposedText) == strings.TrimSpace(s.SourceText)) {
			addFinding(open, rc.ID, s.ID, RuleSensitiveRaw, "敏感片段尚未完成遮蔽或改写", now)
		}
		if leaksSensitiveMarker(s.ProposedText) {
			addFinding(open, rc.ID, s.ID, RuleMaskLeak, "拟公开文本仍包含可识别敏感标记", now)
		}
	}
	result := make([]domain.ReviewFinding, 0, len(open)+len(rc.Findings))
	for _, old := range rc.Findings {
		if !full && !targets[old.SegmentID] {
			continue
		}
		key := findingKey(old.SegmentID, old.RuleCode)
		if current, ok := open[key]; ok {
			current.ID = old.ID
			current.CreatedAt = old.CreatedAt
			current.Remediation = old.Remediation
			current.ReviewerNote = old.ReviewerNote
			current.Returned = old.Returned
			open[key] = current
		} else if old.Status == domain.FindingOpen {
			old.Status = domain.FindingClosed
			closed := domain.UTC(now)
			old.ClosedAt = &closed
			result = append(result, old)
		} else {
			result = append(result, old)
		}
	}
	for _, f := range open {
		result = append(result, f)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SegmentID == result[j].SegmentID {
			return result[i].RuleCode < result[j].RuleCode
		}
		return result[i].SegmentID < result[j].SegmentID
	})
	return result
}

func applicableConsents(all []domain.ConsentGrant, segmentID string) []domain.ConsentGrant {
	var out []domain.ConsentGrant
	for _, g := range all {
		for _, scope := range g.Scope {
			if scope == "*" || scope == segmentID {
				out = append(out, g)
				break
			}
		}
	}
	return out
}

func addFinding(dst map[string]domain.ReviewFinding, caseID, segmentID, code, message string, now time.Time) {
	key := findingKey(segmentID, code)
	dst[key] = domain.ReviewFinding{ID: stableFindingID(caseID, segmentID, code), CaseID: caseID, SegmentID: segmentID, RuleCode: code, Severity: domain.SeverityBlocker, Status: domain.FindingOpen, ReviewerNote: message, CreatedAt: domain.UTC(now)}
}

func findingKey(segmentID, code string) string { return segmentID + "\x00" + code }
func stableFindingID(caseID, segmentID, code string) string {
	return fmt.Sprintf("finding-%x", digest([]byte(caseID + "|" + segmentID + "|" + code))[:8])
}
func leaksSensitiveMarker(text string) bool {
	markers := []string{"身份证", "手机号", "家庭住址", "[未遮蔽]", "SECRET:"}
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
