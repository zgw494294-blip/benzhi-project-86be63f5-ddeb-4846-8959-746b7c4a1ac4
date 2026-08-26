package httpui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"oral-history-release-studio/internal/domain"
)

const maxBodyBytes = 1 << 20

type errorResponse struct {
	Error          string              `json:"error"`
	CurrentVersion int64               `json:"currentVersion,omitempty"`
	Details        []domain.FieldError `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return domain.FieldError{Field: "request", Message: "JSON 请求格式无效：" + err.Error()}
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.FieldError{Field: "request", Message: "请求体只能包含一个 JSON 对象"}
	}
	return nil
}

func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrIdempotency), errors.Is(err, domain.ErrTokenNotFound), errors.Is(err, domain.ErrTokenExpired), errors.Is(err, domain.ErrTokenUsed), errors.Is(err, domain.ErrTokenMismatch):
		return http.StatusConflict
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrInvalidState), errors.Is(err, domain.ErrAlreadyApproved):
		return http.StatusUnprocessableEntity
	default:
		var validation domain.ValidationErrors
		if errors.As(err, &validation) {
			return http.StatusBadRequest
		}
		var field domain.FieldError
		if errors.As(err, &field) {
			return http.StatusBadRequest
		}
		return http.StatusInternalServerError
	}
}
