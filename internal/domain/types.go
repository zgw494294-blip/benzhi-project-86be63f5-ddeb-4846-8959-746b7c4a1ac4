package domain

import "time"

type CaseState string

const (
	StateDraft       CaseState = "draft"
	StateFrozen      CaseState = "frozen"
	StateRemediation CaseState = "remediation"
	StateApproved    CaseState = "approved"
)

type SensitivityTag string

const (
	SensitivityNone       SensitivityTag = "none"
	SensitivityPersonal   SensitivityTag = "personal"
	SensitivityRestricted SensitivityTag = "restricted"
)

type FindingStatus string

const (
	FindingOpen   FindingStatus = "open"
	FindingClosed FindingStatus = "closed"
)

type Severity string

const (
	SeverityBlocker Severity = "blocker"
	SeverityWarning Severity = "warning"
)

func UTC(t time.Time) time.Time { return t.UTC().Truncate(time.Millisecond) }
