package httpui

import (
	"errors"
	"net/http"
	"strings"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
)

func (s *Server) fail(w http.ResponseWriter, r *http.Request, caseID string, err error) {
	version := int64(0)
	if caseID != "" {
		if v, getErr := s.service.GetCase(r.Context(), caseID); getErr == nil {
			version = v.Case.Version
		}
	}
	response := errorResponse{Error: err.Error(), CurrentVersion: version}
	var validation domain.ValidationErrors
	if errors.As(err, &validation) {
		response.Details = validation.Items
	}
	writeJSON(w, statusFor(err), response)
}
func idempotencyKey(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("Idempotency-Key"))
}

func (s *Server) HandleListCases(w http.ResponseWriter, r *http.Request) {
	out, err := s.service.ListCases(r.Context())
	if err != nil {
		s.fail(w, r, "", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleCreateCase(w http.ResponseWriter, r *http.Request) {
	var cmd application.CreateCaseCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, "", err)
		return
	}
	out, err := s.service.CreateCase(r.Context(), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, "", err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (s *Server) HandleGetCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	out, err := s.service.GetCase(r.Context(), id)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleUpdateCase(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.MetadataCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.UpdateMetadata(r.Context(), id, idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleAddSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.SegmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.AddSegment(r.Context(), id, idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (s *Server) HandleAddSegmentsBatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.BatchSegmentsCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.AddSegmentsBatch(r.Context(), id, idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (s *Server) HandleUpdateSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.SegmentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.UpdateSegment(r.Context(), id, r.PathValue("segmentId"), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleDeleteSegment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.VersionCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.DeleteSegment(r.Context(), id, r.PathValue("segmentId"), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleAddConsent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.ConsentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.AddConsent(r.Context(), id, idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
func (s *Server) HandleUpdateConsent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.ConsentCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.UpdateConsent(r.Context(), id, r.PathValue("consentId"), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) versionAction(w http.ResponseWriter, r *http.Request, action func(application.VersionCommand) (application.CaseView, error)) {
	id := r.PathValue("caseId")
	var cmd application.VersionCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := action(cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleFreeze(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	s.versionAction(w, r, func(cmd application.VersionCommand) (application.CaseView, error) {
		return s.service.Freeze(r.Context(), id, idempotencyKey(r), cmd)
	})
}
func (s *Server) HandleChecks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	s.versionAction(w, r, func(cmd application.VersionCommand) (application.CaseView, error) {
		return s.service.RunChecks(r.Context(), id, idempotencyKey(r), cmd, nil)
	})
}
func (s *Server) HandleTargetedCheck(w http.ResponseWriter, r *http.Request) {
	id, segmentID := r.PathValue("caseId"), r.PathValue("segmentId")
	s.versionAction(w, r, func(cmd application.VersionCommand) (application.CaseView, error) {
		return s.service.RunChecks(r.Context(), id, idempotencyKey(r), cmd, []string{segmentID})
	})
}
func (s *Server) HandleReturnFinding(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.ReturnCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.ReturnFinding(r.Context(), id, r.PathValue("findingId"), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleReturnFindingsBatch(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.BatchReturnCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.ReturnFindingsBatch(r.Context(), id, idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleFindingsQuery(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	query := application.FindingQuery{Status: domain.FindingStatus(r.URL.Query().Get("status")), RuleCode: r.URL.Query().Get("ruleCode"), Severity: domain.Severity(r.URL.Query().Get("severity")), SegmentID: r.URL.Query().Get("segmentId"), SensitivityTag: domain.SensitivityTag(r.URL.Query().Get("sensitivityTag"))}
	out, err := s.service.QueryFindings(r.Context(), id, query)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleRemediation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.RemediationCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.Remediate(r.Context(), id, r.PathValue("segmentId"), idempotencyKey(r), cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	s.versionAction(w, r, func(cmd application.VersionCommand) (application.CaseView, error) {
		return s.service.Approve(r.Context(), id, idempotencyKey(r), cmd)
	})
}
func (s *Server) HandleApprovalPreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	var cmd application.VersionCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		s.fail(w, r, id, err)
		return
	}
	out, err := s.service.PreviewApproval(r.Context(), id, cmd)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
func (s *Server) HandleCredential(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("caseId")
	out, err := s.service.VerifyCredential(r.Context(), id)
	if err != nil {
		s.fail(w, r, id, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
