package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ReleaseCase struct {
	ID                 string              `json:"id"`
	Title              string              `json:"title"`
	IntervieweeRef     string              `json:"intervieweeRef"`
	IntendedUse        string              `json:"intendedUse"`
	State              CaseState           `json:"state"`
	Version            int64               `json:"version"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
	FrozenAt           *time.Time          `json:"frozenAt,omitempty"`
	FullCheckCompleted bool                `json:"fullCheckCompleted"`
	Segments           []TranscriptSegment `json:"segments"`
	Consents           []ConsentGrant      `json:"consents"`
	Findings           []ReviewFinding     `json:"findings"`
	Revisions          []SegmentRevision   `json:"revisions"`
	Credential         *ReleaseCredential  `json:"credential,omitempty"`
	Events             []Event             `json:"events"`
	EventSequence      int64               `json:"eventSequence"`
}

func NewReleaseCase(id, title, intervieweeRef, intendedUse, actor string, now time.Time) (*ReleaseCase, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(intervieweeRef) == "" || strings.TrimSpace(intendedUse) == "" {
		return nil, FieldError{Field: "case", Message: "标题、受访者标识和用途说明均不能为空"}
	}
	now = UTC(now)
	c := &ReleaseCase{ID: id, Title: strings.TrimSpace(title), IntervieweeRef: strings.TrimSpace(intervieweeRef), IntendedUse: strings.TrimSpace(intendedUse), State: StateDraft, Version: 1, CreatedAt: now, UpdatedAt: now, Segments: []TranscriptSegment{}, Consents: []ConsentGrant{}, Findings: []ReviewFinding{}, Revisions: []SegmentRevision{}, Events: []Event{}}
	c.record("case.created", actor, now, nil)
	return c, nil
}

func (c *ReleaseCase) ensureMutableDraft() error {
	if c.State == StateApproved {
		return ErrAlreadyApproved
	}
	if c.State != StateDraft {
		return ErrInvalidState
	}
	return nil
}

func (c *ReleaseCase) bump(now time.Time) { c.Version++; c.UpdatedAt = UTC(now) }

func (c *ReleaseCase) UpdateMetadata(title, intervieweeRef, intendedUse, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(intervieweeRef) == "" || strings.TrimSpace(intendedUse) == "" {
		return FieldError{Field: "metadata", Message: "必填字段不能为空"}
	}
	c.Title, c.IntervieweeRef, c.IntendedUse = strings.TrimSpace(title), strings.TrimSpace(intervieweeRef), strings.TrimSpace(intendedUse)
	c.bump(now)
	c.record("case.metadata_updated", actor, now, nil)
	return nil
}

func (c *ReleaseCase) ValidateTimeline() error {
	ordered := append([]TranscriptSegment(nil), c.Segments...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].StartMillis < ordered[j].StartMillis })
	for i, s := range ordered {
		if s.StartMillis < 0 || s.EndMillis <= s.StartMillis {
			return FieldError{Field: "timeRange", Message: "片段结束时间必须大于开始时间"}
		}
		if i > 0 && ordered[i-1].EndMillis > s.StartMillis {
			return FieldError{Field: "timeRange", Message: "片段时间范围不能重叠"}
		}
	}
	return nil
}

func (c *ReleaseCase) Freeze(actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	if len(c.Segments) == 0 {
		return FieldError{Field: "segments", Message: "冻结前至少需要一个片段"}
	}
	if err := c.ValidateTimeline(); err != nil {
		return err
	}
	coverage := c.ConsentCoverageAt(now)
	if !coverage.CanFreeze {
		items := make([]FieldError, 0)
		for _, item := range coverage.Segments {
			if item.Status != CoverageCovered {
				items = append(items, FieldError{Field: "segments." + item.SegmentID, Message: item.Reason})
			}
		}
		return ValidationErrors{Items: items}
	}
	t := UTC(now)
	c.State = StateFrozen
	c.FrozenAt = &t
	c.bump(now)
	c.record("case.frozen", actor, now, map[string]any{"segments": len(c.Segments)})
	return nil
}

func (c *ReleaseCase) OpenBlockers() int {
	n := 0
	for _, f := range c.Findings {
		if f.Severity == SeverityBlocker && f.Status == FindingOpen {
			n++
		}
	}
	return n
}

func (c *ReleaseCase) SegmentByID(id string) (*TranscriptSegment, error) {
	for i := range c.Segments {
		if c.Segments[i].ID == id {
			return &c.Segments[i], nil
		}
	}
	return nil, fmt.Errorf("%w: 片段 %s", ErrNotFound, id)
}
