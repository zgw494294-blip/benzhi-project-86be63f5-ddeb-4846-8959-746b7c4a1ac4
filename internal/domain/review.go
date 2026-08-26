package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type FindingReturn struct {
	FindingID string `json:"findingId"`
	Note      string `json:"note"`
}

func (c *ReleaseCase) SetFindings(findings []ReviewFinding, targeted map[string]bool, actor string, now time.Time) error {
	if c.State != StateFrozen && c.State != StateRemediation {
		return ErrInvalidState
	}
	if targeted == nil {
		c.Findings = findings
		c.FullCheckCompleted = true
	} else {
		for segmentID := range targeted {
			c.annotateLatestRevision(segmentID, findings, now)
		}
		kept := make([]ReviewFinding, 0, len(c.Findings)+len(findings))
		for _, old := range c.Findings {
			if !targeted[old.SegmentID] {
				kept = append(kept, old)
			}
		}
		c.Findings = append(kept, findings...)
	}
	c.bump(now)
	c.record("review.checked", actor, now, map[string]any{"targeted": targeted != nil, "findings": len(findings)})
	return nil
}

func (c *ReleaseCase) annotateLatestRevision(segmentID string, findings []ReviewFinding, now time.Time) {
	before := map[string]bool{}
	for _, finding := range c.Findings {
		if finding.SegmentID == segmentID && finding.Status == FindingOpen {
			before[finding.RuleCode] = true
		}
	}
	after := map[string]bool{}
	for _, finding := range findings {
		if finding.SegmentID == segmentID && finding.Status == FindingOpen {
			after[finding.RuleCode] = true
		}
	}
	for index := len(c.Revisions) - 1; index >= 0; index-- {
		revision := &c.Revisions[index]
		if revision.SegmentID != segmentID || revision.RecheckedAt != nil {
			continue
		}
		for code := range before {
			if after[code] {
				revision.RemainingRuleCodes = append(revision.RemainingRuleCodes, code)
			} else {
				revision.ClosedRuleCodes = append(revision.ClosedRuleCodes, code)
			}
		}
		for code := range after {
			if !before[code] {
				revision.NewRuleCodes = append(revision.NewRuleCodes, code)
			}
		}
		sort.Strings(revision.ClosedRuleCodes)
		sort.Strings(revision.RemainingRuleCodes)
		sort.Strings(revision.NewRuleCodes)
		t := UTC(now)
		revision.RecheckedAt = &t
		return
	}
}

func (c *ReleaseCase) ReturnFindingsBatch(items []FindingReturn, actor string, now time.Time) error {
	if c.State != StateFrozen && c.State != StateRemediation {
		return ErrInvalidState
	}
	if len(items) == 0 {
		return FieldError{Field: "items", Message: "批量退回列表不能为空"}
	}
	byID := make(map[string]*ReviewFinding, len(c.Findings))
	for index := range c.Findings {
		byID[c.Findings[index].ID] = &c.Findings[index]
	}
	seen := map[string]bool{}
	errs := make([]FieldError, 0)
	for index, item := range items {
		id, note := strings.TrimSpace(item.FindingID), strings.TrimSpace(item.Note)
		field := fmt.Sprintf("items[%d]", index)
		if id == "" {
			errs = append(errs, FieldError{Field: field + ".findingId", Message: fmt.Sprintf("第 %d 项发现标识不能为空", index+1)})
			continue
		}
		if seen[id] {
			errs = append(errs, FieldError{Field: field + ".findingId", Message: fmt.Sprintf("第 %d 项发现标识重复", index+1)})
			continue
		}
		seen[id] = true
		finding, ok := byID[id]
		if !ok || finding.CaseID != c.ID {
			errs = append(errs, FieldError{Field: field + ".findingId", Message: fmt.Sprintf("第 %d 项发现不存在或不属于当前案卷", index+1)})
			continue
		}
		if finding.Status != FindingOpen {
			errs = append(errs, FieldError{Field: field + ".findingId", Message: fmt.Sprintf("第 %d 项发现已关闭", index+1)})
		}
		if finding.Severity != SeverityBlocker {
			errs = append(errs, FieldError{Field: field + ".findingId", Message: fmt.Sprintf("第 %d 项不是阻断发现", index+1)})
		}
		if note == "" {
			errs = append(errs, FieldError{Field: field + ".note", Message: fmt.Sprintf("第 %d 项复核意见不能为空", index+1)})
		}
	}
	if len(errs) > 0 {
		return ValidationErrors{Items: errs}
	}
	segmentSet := map[string]bool{}
	findingIDs := make([]string, 0, len(items))
	for _, item := range items {
		finding := byID[strings.TrimSpace(item.FindingID)]
		finding.ReviewerNote = strings.TrimSpace(item.Note)
		finding.Returned = true
		findingIDs = append(findingIDs, finding.ID)
		segmentSet[finding.SegmentID] = true
	}
	segmentIDs := make([]string, 0, len(segmentSet))
	for id := range segmentSet {
		segmentIDs = append(segmentIDs, id)
	}
	sort.Strings(findingIDs)
	sort.Strings(segmentIDs)
	c.State = StateRemediation
	c.bump(now)
	c.record("findings.batch_returned", actor, now, map[string]any{"findingIds": findingIDs, "segmentIds": segmentIDs, "count": len(findingIDs)})
	return nil
}

func (c *ReleaseCase) ReturnFinding(id, note, actor string, now time.Time) error {
	if c.State != StateFrozen && c.State != StateRemediation {
		return ErrInvalidState
	}
	if strings.TrimSpace(note) == "" {
		return FieldError{Field: "note", Message: "复核退回意见不能为空"}
	}
	for i := range c.Findings {
		if c.Findings[i].ID == id && c.Findings[i].Status == FindingOpen {
			c.Findings[i].ReviewerNote = note
			c.Findings[i].Returned = true
			c.State = StateRemediation
			c.bump(now)
			c.record("finding.returned", actor, now, map[string]any{"findingId": id})
			return nil
		}
	}
	return ErrNotFound
}

func (c *ReleaseCase) Approve(credential ReleaseCredential, actor string, now time.Time) error {
	if c.State == StateApproved || c.Credential != nil {
		return ErrAlreadyApproved
	}
	if c.State != StateFrozen && c.State != StateRemediation {
		return ErrInvalidState
	}
	if c.OpenBlockers() != 0 {
		return FieldError{Field: "findings", Message: "仍有未关闭的阻断项"}
	}
	if !c.FullCheckCompleted {
		return FieldError{Field: "checks", Message: "批准前必须执行全量检查"}
	}
	if len(credential.PublicSegmentIDs) == 0 {
		return FieldError{Field: "manifest", Message: "公开清单不能为空"}
	}
	c.State = StateApproved
	c.bump(now)
	credential.ApprovedVersion = c.Version
	c.Credential = &credential
	c.record("case.approved", actor, now, map[string]any{"credentialId": credential.ID, "manifestHash": credential.ManifestHash})
	return nil
}
