package application

import (
	"context"
	"sort"
	"strings"

	"oral-history-release-studio/internal/assurance"
	"oral-history-release-studio/internal/domain"
)

type FindingQuery struct {
	Status         domain.FindingStatus  `json:"status,omitempty"`
	RuleCode       string                `json:"ruleCode,omitempty"`
	Severity       domain.Severity       `json:"severity,omitempty"`
	SegmentID      string                `json:"segmentId,omitempty"`
	SensitivityTag domain.SensitivityTag `json:"sensitivityTag,omitempty"`
}

type FindingStatistics struct {
	TotalFindings    int            `json:"totalFindings"`
	MatchedTotal     int            `json:"matchedTotal"`
	OpenBlockers     int            `json:"openBlockers"`
	Closed           int            `json:"closed"`
	AffectedSegments int            `json:"affectedSegments"`
	RuleCounts       map[string]int `json:"ruleCounts"`
	StatisticsScope  string         `json:"statisticsScope"`
}

type FindingQueryResult struct {
	CaseID      string                 `json:"caseId"`
	CaseVersion int64                  `json:"caseVersion"`
	Filters     FindingQuery           `json:"filters"`
	Statistics  FindingStatistics      `json:"statistics"`
	Findings    []domain.ReviewFinding `json:"findings"`
}

func (s *Service) QueryFindings(ctx context.Context, caseID string, query FindingQuery) (FindingQueryResult, error) {
	query.RuleCode = strings.TrimSpace(query.RuleCode)
	query.SegmentID = strings.TrimSpace(query.SegmentID)
	if err := validateFindingQuery(query); err != nil {
		return FindingQueryResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return FindingQueryResult{}, err
	}
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return FindingQueryResult{}, err
	}
	sequence := make(map[string]int, len(c.Segments))
	tags := make(map[string]domain.SensitivityTag, len(c.Segments))
	for _, segment := range c.Segments {
		sequence[segment.ID] = segment.Sequence
		tags[segment.ID] = segment.SensitivityTag
	}
	if query.SegmentID != "" {
		if _, exists := sequence[query.SegmentID]; !exists {
			return FindingQueryResult{}, domain.FieldError{Field: "segmentId", Message: "筛选片段标识不存在于当前案卷"}
		}
	}
	matched := make([]domain.ReviewFinding, 0, len(c.Findings))
	stats := FindingStatistics{TotalFindings: len(c.Findings), RuleCounts: map[string]int{}, StatisticsScope: "当前案卷 version 下的筛选匹配结果；totalFindings 为当前版本全部发现数"}
	affected := map[string]bool{}
	for _, finding := range c.Findings {
		if query.Status != "" && finding.Status != query.Status || query.RuleCode != "" && finding.RuleCode != query.RuleCode || query.Severity != "" && finding.Severity != query.Severity || query.SegmentID != "" && finding.SegmentID != query.SegmentID || query.SensitivityTag != "" && tags[finding.SegmentID] != query.SensitivityTag {
			continue
		}
		matched = append(matched, finding)
		stats.RuleCounts[finding.RuleCode]++
		affected[finding.SegmentID] = true
		if finding.Status == domain.FindingClosed {
			stats.Closed++
		}
		if finding.Status == domain.FindingOpen && finding.Severity == domain.SeverityBlocker {
			stats.OpenBlockers++
		}
	}
	sort.SliceStable(matched, func(i, j int) bool {
		left, right := matched[i], matched[j]
		if sequence[left.SegmentID] != sequence[right.SegmentID] {
			return sequence[left.SegmentID] < sequence[right.SegmentID]
		}
		if left.RuleCode != right.RuleCode {
			return left.RuleCode < right.RuleCode
		}
		if !left.CreatedAt.Equal(right.CreatedAt) {
			return left.CreatedAt.Before(right.CreatedAt)
		}
		return left.ID < right.ID
	})
	stats.MatchedTotal = len(matched)
	stats.AffectedSegments = len(affected)
	return FindingQueryResult{CaseID: c.ID, CaseVersion: c.Version, Filters: query, Statistics: stats, Findings: matched}, nil
}

func validateFindingQuery(query FindingQuery) error {
	if query.Status != "" && query.Status != domain.FindingOpen && query.Status != domain.FindingClosed {
		return domain.FieldError{Field: "status", Message: "未知发现状态，只能为 open 或 closed"}
	}
	knownRules := map[string]bool{assurance.RuleMissingConsent: true, assurance.RuleUseConflict: true, assurance.RuleConsentExpired: true, assurance.RuleSensitiveRaw: true, assurance.RuleMaskLeak: true}
	if query.RuleCode != "" && !knownRules[strings.TrimSpace(query.RuleCode)] {
		return domain.FieldError{Field: "ruleCode", Message: "未知检查规则代码"}
	}
	if query.Severity != "" && query.Severity != domain.SeverityBlocker && query.Severity != domain.SeverityWarning {
		return domain.FieldError{Field: "severity", Message: "未知严重级别，只能为 blocker 或 warning"}
	}
	if query.SensitivityTag != "" && query.SensitivityTag != domain.SensitivityNone && query.SensitivityTag != domain.SensitivityPersonal && query.SensitivityTag != domain.SensitivityRestricted {
		return domain.FieldError{Field: "sensitivityTag", Message: "未知敏感性标签"}
	}
	return nil
}
