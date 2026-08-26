package application

import (
	"strings"

	"oral-history-release-studio/internal/domain"
)

type role int

const (
	roleOrganizer role = iota + 1
	roleReviewer
	roleReleaseManager
)

func authorize(actor string, allowed ...role) error {
	parts := strings.SplitN(strings.TrimSpace(actor), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return domain.FieldError{Field: "actor", Message: "身份格式应为 role:name"}
	}
	var got role
	switch parts[0] {
	case "organizer":
		got = roleOrganizer
	case "reviewer":
		got = roleReviewer
	case "release_manager":
		got = roleReleaseManager
	default:
		return domain.ErrForbidden
	}
	for _, r := range allowed {
		if r == got {
			return nil
		}
	}
	return domain.ErrForbidden
}
