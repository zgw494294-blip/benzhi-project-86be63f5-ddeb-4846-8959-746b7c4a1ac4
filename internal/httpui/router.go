package httpui

import (
	"io/fs"
	"net/http"

	"oral-history-release-studio/internal/application"
)

type Server struct {
	service *application.Service
	mux     *http.ServeMux
}

func New(service *application.Service) *Server {
	s := &Server{service: service, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /{$}", s.HandleRoot)
	s.mux.HandleFunc("GET /assets/", s.HandleAssets)
	s.mux.HandleFunc("GET /api/health", s.HandleHealth)
	s.mux.HandleFunc("GET /api/cases", s.HandleListCases)
	s.mux.HandleFunc("POST /api/cases", s.HandleCreateCase)
	s.mux.HandleFunc("GET /api/cases/{caseId}", s.HandleGetCase)
	s.mux.HandleFunc("PUT /api/cases/{caseId}", s.HandleUpdateCase)
	s.mux.HandleFunc("POST /api/cases/{caseId}/segments", s.HandleAddSegment)
	s.mux.HandleFunc("POST /api/cases/{caseId}/segments/batch", s.HandleAddSegmentsBatch)
	s.mux.HandleFunc("PUT /api/cases/{caseId}/segments/{segmentId}", s.HandleUpdateSegment)
	s.mux.HandleFunc("DELETE /api/cases/{caseId}/segments/{segmentId}", s.HandleDeleteSegment)
	s.mux.HandleFunc("POST /api/cases/{caseId}/consents", s.HandleAddConsent)
	s.mux.HandleFunc("PUT /api/cases/{caseId}/consents/{consentId}", s.HandleUpdateConsent)
	s.mux.HandleFunc("POST /api/cases/{caseId}/freeze", s.HandleFreeze)
	s.mux.HandleFunc("POST /api/cases/{caseId}/checks", s.HandleChecks)
	s.mux.HandleFunc("GET /api/cases/{caseId}/findings", s.HandleFindingsQuery)
	s.mux.HandleFunc("POST /api/cases/{caseId}/findings/batch-return", s.HandleReturnFindingsBatch)
	s.mux.HandleFunc("POST /api/cases/{caseId}/findings/{findingId}/return", s.HandleReturnFinding)
	s.mux.HandleFunc("POST /api/cases/{caseId}/segments/{segmentId}/remediation", s.HandleRemediation)
	s.mux.HandleFunc("POST /api/cases/{caseId}/segments/{segmentId}/recheck", s.HandleTargetedCheck)
	s.mux.HandleFunc("POST /api/cases/{caseId}/approve", s.HandleApprove)
	s.mux.HandleFunc("POST /api/cases/{caseId}/approval-preview", s.HandleApprovalPreview)
	s.mux.HandleFunc("GET /api/cases/{caseId}/credential", s.HandleCredential)
	return s
}

func (s *Server) Handler() http.Handler { return securityHeaders(s.mux) }

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) HandleRoot(w http.ResponseWriter, r *http.Request) {
	b, err := assets.ReadFile("assets/index.html")
	if err != nil {
		http.Error(w, "页面资源不可用", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
func (s *Server) HandleAssets(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(assets, "assets")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.StripPrefix("/assets/", http.FileServer(http.FS(sub))).ServeHTTP(w, r)
}
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
