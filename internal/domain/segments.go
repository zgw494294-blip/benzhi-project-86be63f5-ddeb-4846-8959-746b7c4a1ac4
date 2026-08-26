package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type indexedSegment struct {
	segment TranscriptSegment
	row     int
}

func (c *ReleaseCase) AddSegmentsBatch(batch []TranscriptSegment, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	if len(batch) == 0 {
		return FieldError{Field: "segments", Message: "批量片段不能为空"}
	}
	errs := make([]FieldError, 0)
	ids := make(map[string]int, len(c.Segments)+len(batch))
	for _, segment := range c.Segments {
		ids[segment.ID] = 0
	}
	merged := make([]indexedSegment, 0, len(c.Segments)+len(batch))
	for _, segment := range c.Segments {
		merged = append(merged, indexedSegment{segment: segment})
	}
	for index, input := range batch {
		row := index + 1
		input.ID = strings.TrimSpace(input.ID)
		input.SourceText = strings.TrimSpace(input.SourceText)
		input.ProposedText = strings.TrimSpace(input.ProposedText)
		if input.ID == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].id", index), Message: fmt.Sprintf("第 %d 行片段标识不能为空", row)})
		} else if previous, exists := ids[input.ID]; exists {
			message := fmt.Sprintf("第 %d 行片段标识重复", row)
			if previous == 0 {
				message = fmt.Sprintf("第 %d 行片段标识已存在于案卷", row)
			}
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].id", index), Message: message})
		} else {
			ids[input.ID] = row
		}
		if input.StartMillis < 0 {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].startMillis", index), Message: fmt.Sprintf("第 %d 行开始时间不能小于 0", row)})
		}
		if input.EndMillis <= input.StartMillis {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].endMillis", index), Message: fmt.Sprintf("第 %d 行结束时间必须大于开始时间", row)})
		}
		if input.SourceText == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].sourceText", index), Message: fmt.Sprintf("第 %d 行原始文本不能为空", row)})
		}
		if input.ProposedText == "" {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].proposedText", index), Message: fmt.Sprintf("第 %d 行拟公开文本不能为空", row)})
		}
		if input.SensitivityTag != SensitivityNone && input.SensitivityTag != SensitivityPersonal && input.SensitivityTag != SensitivityRestricted {
			errs = append(errs, FieldError{Field: fmt.Sprintf("segments[%d].sensitivityTag", index), Message: fmt.Sprintf("第 %d 行敏感性标签无效", row)})
		}
		input.CaseID = c.ID
		merged = append(merged, indexedSegment{segment: input, row: row})
	}
	if len(errs) > 0 {
		return ValidationErrors{Items: errs}
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].segment.StartMillis == merged[j].segment.StartMillis {
			return merged[i].segment.ID < merged[j].segment.ID
		}
		return merged[i].segment.StartMillis < merged[j].segment.StartMillis
	})
	for index := 1; index < len(merged); index++ {
		if merged[index-1].segment.EndMillis <= merged[index].segment.StartMillis {
			continue
		}
		bad := merged[index]
		if bad.row == 0 {
			bad = merged[index-1]
		}
		fieldIndex := bad.row - 1
		return ValidationErrors{Items: []FieldError{{Field: fmt.Sprintf("segments[%d].timeRange", fieldIndex), Message: fmt.Sprintf("第 %d 行时间范围与片段 %s 重叠", bad.row, otherSegmentID(merged[index-1], merged[index], bad.segment.ID))}}}
	}
	c.Segments = make([]TranscriptSegment, len(merged))
	addedIDs := make([]string, 0, len(batch))
	for index := range merged {
		merged[index].segment.Sequence = index + 1
		c.Segments[index] = merged[index].segment
		if merged[index].row > 0 {
			addedIDs = append(addedIDs, merged[index].segment.ID)
		}
	}
	sort.Strings(addedIDs)
	c.bump(now)
	c.record("segments.batch_added", actor, now, map[string]any{"segmentIds": addedIDs, "count": len(addedIDs)})
	return nil
}

func otherSegmentID(a, b indexedSegment, badID string) string {
	if a.segment.ID != badID {
		return a.segment.ID
	}
	return b.segment.ID
}

func (c *ReleaseCase) AddSegment(s TranscriptSegment, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(s.SourceText) == "" {
		return FieldError{Field: "segment", Message: "片段 ID 和原始文本不能为空"}
	}
	if _, err := c.SegmentByID(s.ID); err == nil {
		return FieldError{Field: "segmentId", Message: "片段 ID 已存在"}
	}
	s.CaseID = c.ID
	s.SourceText = strings.TrimSpace(s.SourceText)
	s.ProposedText = strings.TrimSpace(s.ProposedText)
	if s.ProposedText == "" {
		s.ProposedText = s.SourceText
	}
	if s.SensitivityTag == "" {
		s.SensitivityTag = SensitivityNone
	}
	c.Segments = append(c.Segments, s)
	if err := c.ValidateTimeline(); err != nil {
		c.Segments = c.Segments[:len(c.Segments)-1]
		return err
	}
	c.resequence()
	c.bump(now)
	c.record("segment.added", actor, now, map[string]any{"segmentId": s.ID})
	return nil
}

func (c *ReleaseCase) UpdateSegment(s TranscriptSegment, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	existing, err := c.SegmentByID(s.ID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(s.SourceText) == "" {
		return FieldError{Field: "sourceText", Message: "原始文本不能为空"}
	}
	s.CaseID = c.ID
	s.SourceText = strings.TrimSpace(s.SourceText)
	s.ProposedText = strings.TrimSpace(s.ProposedText)
	if s.ProposedText == "" {
		s.ProposedText = s.SourceText
	}
	old := *existing
	*existing = s
	if err := c.ValidateTimeline(); err != nil {
		*existing = old
		return err
	}
	c.resequence()
	c.bump(now)
	c.record("segment.updated", actor, now, map[string]any{"segmentId": s.ID})
	return nil
}

func (c *ReleaseCase) DeleteSegment(id, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	for i := range c.Segments {
		if c.Segments[i].ID == id {
			c.Segments = append(c.Segments[:i], c.Segments[i+1:]...)
			c.resequence()
			c.bump(now)
			c.record("segment.deleted", actor, now, map[string]any{"segmentId": id})
			return nil
		}
	}
	return ErrNotFound
}

func (c *ReleaseCase) resequence() {
	sort.SliceStable(c.Segments, func(i, j int) bool { return c.Segments[i].StartMillis < c.Segments[j].StartMillis })
	for i := range c.Segments {
		c.Segments[i].Sequence = i + 1
	}
}

func (c *ReleaseCase) RemediateSegment(id, proposed, explanation, actor string, now time.Time) error {
	if c.State != StateRemediation && c.State != StateFrozen {
		return ErrInvalidState
	}
	if strings.TrimSpace(proposed) == "" || strings.TrimSpace(explanation) == "" {
		return FieldError{Field: "remediation", Message: "拟公开文本和整改说明不能为空"}
	}
	s, err := c.SegmentByID(id)
	if err != nil {
		return err
	}
	proposed = strings.TrimSpace(proposed)
	if proposed == strings.TrimSpace(s.ProposedText) {
		return FieldError{Field: "proposedText", Message: "拟公开文本未发生实际变化"}
	}
	relatedIDs := []string{}
	relatedRules := []string{}
	relatedReviews := []RevisionFindingReference{}
	ruleSeen := map[string]bool{}
	for _, finding := range c.Findings {
		if finding.SegmentID == id && finding.Status == FindingOpen && finding.Returned {
			relatedIDs = append(relatedIDs, finding.ID)
			relatedReviews = append(relatedReviews, RevisionFindingReference{FindingID: finding.ID, RuleCode: finding.RuleCode, ReviewerNote: finding.ReviewerNote})
			if !ruleSeen[finding.RuleCode] {
				relatedRules = append(relatedRules, finding.RuleCode)
				ruleSeen[finding.RuleCode] = true
			}
		}
	}
	if len(relatedIDs) == 0 {
		// 兼容由领域层直接驱动的旧流程；公开应用入口会严格要求 Returned。
		for _, finding := range c.Findings {
			if finding.SegmentID == id && finding.Status == FindingOpen {
				relatedIDs = append(relatedIDs, finding.ID)
				relatedReviews = append(relatedReviews, RevisionFindingReference{FindingID: finding.ID, RuleCode: finding.RuleCode, ReviewerNote: finding.ReviewerNote})
				if !ruleSeen[finding.RuleCode] {
					relatedRules = append(relatedRules, finding.RuleCode)
					ruleSeen[finding.RuleCode] = true
				}
			}
		}
	}
	if len(relatedIDs) == 0 {
		return FieldError{Field: "segmentId", Message: "仅可整改存在已退回开放发现的片段"}
	}
	sort.Strings(relatedIDs)
	sort.Strings(relatedRules)
	before := s.ProposedText
	s.ProposedText = proposed
	s.Disposition = strings.TrimSpace(explanation)
	c.State = StateRemediation
	for i := range c.Findings {
		if c.Findings[i].SegmentID == id && c.Findings[i].Status == FindingOpen {
			c.Findings[i].Remediation = explanation
		}
	}
	submittedVersion := c.Version
	c.bump(now)
	revisionID := fmt.Sprintf("revision-%s-%d", id, c.Version)
	c.Revisions = append(c.Revisions, SegmentRevision{ID: revisionID, SegmentID: id, BeforeText: before, AfterText: proposed, BeforeDigest: textDigest(before), AfterDigest: textDigest(proposed), Explanation: strings.TrimSpace(explanation), Actor: actor, CreatedAt: UTC(now), SubmittedVersion: submittedVersion, RelatedFindingIDs: relatedIDs, RelatedRuleCodes: relatedRules, RelatedReviews: relatedReviews})
	c.record("segment.remediated", actor, now, map[string]any{"segmentId": id, "revisionId": revisionID, "explanation": explanation})
	return nil
}

func (c *ReleaseCase) HasReturnedOpenFinding(segmentID string) bool {
	for _, finding := range c.Findings {
		if finding.SegmentID == segmentID && finding.Status == FindingOpen && finding.Returned {
			return true
		}
	}
	return false
}

func textDigest(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}
