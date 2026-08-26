package domain

import (
	"strings"
	"time"
)

func (c *ReleaseCase) AddConsent(g ConsentGrant, actor string, now time.Time) error {
	if c.State == StateApproved {
		return ErrAlreadyApproved
	}
	if c.State != StateDraft && c.State != StateFrozen && c.State != StateRemediation {
		return ErrInvalidState
	}
	if strings.TrimSpace(g.ID) == "" || strings.TrimSpace(g.SignedBy) == "" || len(g.Scope) == 0 || len(g.AllowedUses) == 0 {
		return FieldError{Field: "consent", Message: "授权 ID、范围、用途和签署人不能为空"}
	}
	if !g.ExpiresAt.After(g.ValidFrom) {
		return FieldError{Field: "expiresAt", Message: "授权到期时间必须晚于生效时间"}
	}
	for _, old := range c.Consents {
		if old.ID == g.ID {
			return FieldError{Field: "consentId", Message: "授权 ID 已存在"}
		}
	}
	g.CaseID = c.ID
	g.ValidFrom = UTC(g.ValidFrom)
	g.ExpiresAt = UTC(g.ExpiresAt)
	g.SignedAt = UTC(g.SignedAt)
	c.Consents = append(c.Consents, g)
	if c.State != StateDraft {
		c.State = StateRemediation
	}
	c.bump(now)
	kind := "consent.added"
	if c.State == StateRemediation {
		kind = "consent.supplemented"
	}
	c.record(kind, actor, now, map[string]any{"consentId": g.ID})
	return nil
}

func (c *ReleaseCase) ReplaceConsent(g ConsentGrant, actor string, now time.Time) error {
	if err := c.ensureMutableDraft(); err != nil {
		return err
	}
	for i := range c.Consents {
		if c.Consents[i].ID == g.ID {
			if strings.TrimSpace(g.SignedBy) == "" || len(g.Scope) == 0 || len(g.AllowedUses) == 0 {
				return FieldError{Field: "consent", Message: "授权范围、用途和签署人不能为空"}
			}
			if !g.ExpiresAt.After(g.ValidFrom) {
				return FieldError{Field: "expiresAt", Message: "授权期限无效"}
			}
			g.CaseID = c.ID
			g.ValidFrom = UTC(g.ValidFrom)
			g.ExpiresAt = UTC(g.ExpiresAt)
			g.SignedAt = UTC(g.SignedAt)
			c.Consents[i] = g
			c.bump(now)
			c.record("consent.updated", actor, now, map[string]any{"consentId": g.ID})
			return nil
		}
	}
	return ErrNotFound
}
