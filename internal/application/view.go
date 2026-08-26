package application

import (
	"oral-history-release-studio/internal/assurance"
	"oral-history-release-studio/internal/domain"
)

type CaseView struct {
	Case                   *domain.ReleaseCase               `json:"case"`
	OpenBlockers           int                               `json:"openBlockers"`
	AllowedActions         []string                          `json:"allowedActions"`
	CredentialValid        *bool                             `json:"credentialValid,omitempty"`
	ValidationMessage      string                            `json:"validationMessage,omitempty"`
	ConsentCoverage        domain.ConsentCoverage            `json:"consentCoverage"`
	CredentialVerification *assurance.CredentialVerification `json:"credentialVerification,omitempty"`
}

func buildView(c *domain.ReleaseCase, coverage domain.ConsentCoverage) CaseView {
	actions := []string{}
	switch c.State {
	case domain.StateDraft:
		actions = []string{"edit", "freeze"}
	case domain.StateFrozen:
		actions = []string{"check", "return", "approve", "add_consent"}
	case domain.StateRemediation:
		actions = []string{"remediate", "targeted_check", "approve", "add_consent"}
	case domain.StateApproved:
		actions = []string{"verify_credential"}
	}
	if c.State == domain.StateDraft && !coverage.CanFreeze {
		filtered := actions[:0]
		for _, action := range actions {
			if action != "freeze" {
				filtered = append(filtered, action)
			}
		}
		actions = filtered
	}
	return CaseView{Case: c, OpenBlockers: c.OpenBlockers(), AllowedActions: actions, ConsentCoverage: coverage}
}
