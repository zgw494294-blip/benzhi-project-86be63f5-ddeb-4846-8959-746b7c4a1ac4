package domain

import (
	"sort"
	"strings"
	"time"
)

type CoverageStatus string

const (
	CoverageCovered     CoverageStatus = "covered"
	CoverageNotYetValid CoverageStatus = "not_yet_valid"
	CoverageExpired     CoverageStatus = "expired"
	CoverageUseConflict CoverageStatus = "use_conflict"
	CoverageMissing     CoverageStatus = "missing"
)

type SegmentCoverage struct {
	SegmentID string         `json:"segmentId"`
	Sequence  int            `json:"sequence"`
	Status    CoverageStatus `json:"status"`
	ConsentID string         `json:"consentId,omitempty"`
	Reason    string         `json:"reason"`
}

type OrphanConsentScope struct {
	ConsentID string `json:"consentId"`
	SegmentID string `json:"segmentId"`
}

type ConsentCoverage struct {
	Segments    []SegmentCoverage    `json:"segments"`
	OrphanScope []OrphanConsentScope `json:"orphanScope"`
	CanFreeze   bool                 `json:"canFreeze"`
}

// ConsentCoverageAt 是冻结与详情预览共享的无副作用授权判定。
func (c *ReleaseCase) ConsentCoverageAt(now time.Time) ConsentCoverage {
	now = UTC(now)
	segments := append([]TranscriptSegment(nil), c.Segments...)
	sort.SliceStable(segments, func(i, j int) bool { return segments[i].Sequence < segments[j].Sequence })
	known := make(map[string]bool, len(segments))
	for _, segment := range segments {
		known[segment.ID] = true
	}
	consents := append([]ConsentGrant(nil), c.Consents...)
	sort.SliceStable(consents, func(i, j int) bool { return consents[i].ID < consents[j].ID })
	result := ConsentCoverage{CanFreeze: len(segments) > 0}
	for _, segment := range segments {
		result.Segments = append(result.Segments, coverageForSegment(segment, consents, c.IntendedUse, now))
		if result.Segments[len(result.Segments)-1].Status != CoverageCovered {
			result.CanFreeze = false
		}
	}
	for _, grant := range consents {
		for _, scope := range grant.Scope {
			scope = strings.TrimSpace(scope)
			if scope != "" && scope != "*" && !known[scope] {
				result.OrphanScope = append(result.OrphanScope, OrphanConsentScope{ConsentID: grant.ID, SegmentID: scope})
			}
		}
	}
	sort.SliceStable(result.OrphanScope, func(i, j int) bool {
		if result.OrphanScope[i].ConsentID == result.OrphanScope[j].ConsentID {
			return result.OrphanScope[i].SegmentID < result.OrphanScope[j].SegmentID
		}
		return result.OrphanScope[i].ConsentID < result.OrphanScope[j].ConsentID
	})
	return result
}

func coverageForSegment(segment TranscriptSegment, consents []ConsentGrant, intendedUse string, now time.Time) SegmentCoverage {
	base := SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageMissing, Reason: "缺失覆盖该片段的授权"}
	var candidates []ConsentGrant
	for _, grant := range consents {
		if scopeContains(grant.Scope, segment.ID) {
			candidates = append(candidates, grant)
		}
	}
	if len(candidates) == 0 {
		return base
	}
	for _, grant := range candidates {
		if now.Before(grant.ValidFrom) || now.After(grant.ExpiresAt) || !useAllowed(grant, intendedUse) {
			continue
		}
		return SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageCovered, ConsentID: grant.ID, Reason: "当前有效授权已覆盖片段和公开用途"}
	}
	// 未通过时采用稳定优先级和稳定授权 ID，使重复预览结果一致。
	for _, grant := range candidates {
		if !now.Before(grant.ValidFrom) && !now.After(grant.ExpiresAt) {
			return SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageUseConflict, ConsentID: grant.ID, Reason: "覆盖授权不允许当前公开用途或包含禁止公开限制"}
		}
	}
	for _, grant := range candidates {
		if now.Before(grant.ValidFrom) {
			return SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageNotYetValid, ConsentID: grant.ID, Reason: "覆盖授权尚未生效"}
		}
	}
	for _, grant := range candidates {
		if now.After(grant.ExpiresAt) {
			return SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageExpired, ConsentID: grant.ID, Reason: "覆盖授权已到期"}
		}
	}
	return SegmentCoverage{SegmentID: segment.ID, Sequence: segment.Sequence, Status: CoverageMissing, ConsentID: candidates[0].ID, Reason: "缺失当前有效授权"}
}

func scopeContains(scope []string, segmentID string) bool {
	for _, value := range scope {
		if strings.TrimSpace(value) == "*" || strings.TrimSpace(value) == segmentID {
			return true
		}
	}
	return false
}

func useAllowed(grant ConsentGrant, intendedUse string) bool {
	for _, restriction := range grant.Restrictions {
		value := strings.ToLower(strings.TrimSpace(restriction))
		if strings.Contains(restriction, "禁止公开") || strings.Contains(value, "no-public") {
			return false
		}
	}
	for _, use := range grant.AllowedUses {
		if strings.TrimSpace(use) == "*" || strings.EqualFold(strings.TrimSpace(use), strings.TrimSpace(intendedUse)) {
			return true
		}
	}
	return false
}
