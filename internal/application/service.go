package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"oral-history-release-studio/internal/assurance"
	"oral-history-release-studio/internal/domain"
)

type Service struct {
	repo    Repository
	checker *assurance.Checker
	now     func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, checker: assurance.NewChecker(), now: time.Now}
}
func (s *Service) WithClock(clock func() time.Time) *Service { s.now = clock; return s }
func (s *Service) view(c *domain.ReleaseCase) CaseView {
	return buildView(c, c.ConsentCoverageAt(s.now()))
}

func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return prefix + "-fallback"
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
func meta(key, action, actor string, cmd any) MutationMeta {
	return MutationMeta{Key: strings.TrimSpace(key), Fingerprint: fingerprint(action, cmd), Actor: actor, Action: action}
}
func requireKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return domain.FieldError{Field: "Idempotency-Key", Message: "写入请求必须提供幂等键"}
	}
	return nil
}

func (s *Service) CreateCase(ctx context.Context, key string, cmd CreateCaseCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	c, err := domain.NewReleaseCase(newID("case"), cmd.Title, cmd.IntervieweeRef, cmd.IntendedUse, cmd.Actor, s.now())
	if err != nil {
		return CaseView{}, err
	}
	saved, _, err := s.repo.Create(ctx, c, meta(key, "create_case", cmd.Actor, cmd))
	if err != nil {
		return CaseView{}, err
	}
	return s.view(saved), nil
}

func (s *Service) GetCase(ctx context.Context, id string) (CaseView, error) {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}
func (s *Service) ListCases(ctx context.Context) ([]CaseView, error) {
	cases, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CaseView, 0, len(cases))
	for _, c := range cases {
		out = append(out, s.view(c))
	}
	return out, nil
}

func (s *Service) UpdateMetadata(ctx context.Context, caseID, key string, cmd MetadataCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "update_metadata", cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		return c.UpdateMetadata(cmd.Title, cmd.IntervieweeRef, cmd.IntendedUse, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) AddSegment(ctx context.Context, caseID, key string, cmd SegmentCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	// 当调用方省略可选片段 ID 时，由服务分配。该分配不得进入幂等指纹，
	// 否则同一请求重放时会因随机 ID 不同而被判定为幂等冲突。指纹仅覆盖调用方提供的值，
	// 分配的 ID 在重放时直接复用首次保存的结果，因此不会重复创建片段。
	m := meta(key, "add_segment", cmd.Actor, cmd)
	if cmd.ID == "" {
		cmd.ID = newID("segment")
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, m, func(c *domain.ReleaseCase) error {
		return c.AddSegment(domain.TranscriptSegment{ID: cmd.ID, StartMillis: cmd.StartMillis, EndMillis: cmd.EndMillis, SourceText: cmd.SourceText, ProposedText: cmd.ProposedText, SensitivityTag: cmd.SensitivityTag}, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) AddSegmentsBatch(ctx context.Context, caseID, key string, cmd BatchSegmentsCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	if len(cmd.Segments) > 100 {
		return CaseView{}, domain.FieldError{Field: "segments", Message: "单次批量录入不能超过 100 个片段"}
	}
	segments := make([]domain.TranscriptSegment, len(cmd.Segments))
	for index, item := range cmd.Segments {
		segments[index] = domain.TranscriptSegment{ID: item.ID, StartMillis: item.StartMillis, EndMillis: item.EndMillis, SourceText: item.SourceText, SensitivityTag: item.SensitivityTag, ProposedText: item.ProposedText}
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "add_segments_batch", cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		return c.AddSegmentsBatch(segments, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) UpdateSegment(ctx context.Context, caseID, segmentID, key string, cmd SegmentCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	cmd.ID = segmentID
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "update_segment", cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		return c.UpdateSegment(domain.TranscriptSegment{ID: segmentID, StartMillis: cmd.StartMillis, EndMillis: cmd.EndMillis, SourceText: cmd.SourceText, ProposedText: cmd.ProposedText, SensitivityTag: cmd.SensitivityTag}, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) DeleteSegment(ctx context.Context, caseID, segmentID, key string, cmd VersionCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "delete_segment:"+segmentID, cmd.Actor, cmd), func(c *domain.ReleaseCase) error { return c.DeleteSegment(segmentID, cmd.Actor, s.now()) })
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) AddConsent(ctx context.Context, caseID, key string, cmd ConsentCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	// 当调用方省略可选授权 ID 时，由服务分配。该分配不得进入幂等指纹，
	// 否则同一请求重放时会因随机 ID 不同而被判定为幂等冲突。指纹仅覆盖调用方提供的值，
	// 分配的 ID 在重放时直接复用首次保存的结果，因此不会重复创建授权。
	m := meta(key, "add_consent", cmd.Actor, cmd)
	if cmd.ID == "" {
		cmd.ID = newID("consent")
	}
	g := domain.ConsentGrant{ID: cmd.ID, Scope: cmd.Scope, AllowedUses: cmd.AllowedUses, Restrictions: cmd.Restrictions, ValidFrom: cmd.ValidFrom, ExpiresAt: cmd.ExpiresAt, SignedBy: cmd.SignedBy, SignedAt: cmd.SignedAt}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, m, func(c *domain.ReleaseCase) error { return c.AddConsent(g, cmd.Actor, s.now()) })
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) UpdateConsent(ctx context.Context, caseID, consentID, key string, cmd ConsentCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	cmd.ID = consentID
	g := domain.ConsentGrant{ID: cmd.ID, Scope: cmd.Scope, AllowedUses: cmd.AllowedUses, Restrictions: cmd.Restrictions, ValidFrom: cmd.ValidFrom, ExpiresAt: cmd.ExpiresAt, SignedBy: cmd.SignedBy, SignedAt: cmd.SignedAt}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "update_consent", cmd.Actor, cmd), func(c *domain.ReleaseCase) error { return c.ReplaceConsent(g, cmd.Actor, s.now()) })
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) Freeze(ctx context.Context, caseID, key string, cmd VersionCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "freeze", cmd.Actor, cmd), func(c *domain.ReleaseCase) error { return c.Freeze(cmd.Actor, s.now()) })
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) RunChecks(ctx context.Context, caseID, key string, cmd VersionCommand, segmentIDs []string) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer, roleReviewer); err != nil {
		return CaseView{}, err
	}
	action := "full_check"
	if len(segmentIDs) > 0 {
		action = "targeted_check:" + strings.Join(segmentIDs, ",")
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, action, cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		findings := s.checker.Check(c, segmentIDs, s.now())
		var targets map[string]bool
		if len(segmentIDs) > 0 {
			targets = map[string]bool{}
			for _, id := range segmentIDs {
				if _, err := c.SegmentByID(id); err != nil {
					return err
				}
				targets[id] = true
			}
		}
		return c.SetFindings(findings, targets, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) ReturnFinding(ctx context.Context, caseID, findingID, key string, cmd ReturnCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleReviewer); err != nil {
		return CaseView{}, err
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "return_finding:"+findingID, cmd.Actor, cmd), func(c *domain.ReleaseCase) error { return c.ReturnFinding(findingID, cmd.Note, cmd.Actor, s.now()) })
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) ReturnFindingsBatch(ctx context.Context, caseID, key string, cmd BatchReturnCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleReviewer); err != nil {
		return CaseView{}, err
	}
	if len(cmd.Items) > 100 {
		return CaseView{}, domain.FieldError{Field: "items", Message: "单次批量退回不能超过 100 项发现"}
	}
	items := make([]domain.FindingReturn, len(cmd.Items))
	for index, item := range cmd.Items {
		items[index] = domain.FindingReturn{FindingID: item.FindingID, Note: item.Note}
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "return_findings_batch", cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		return c.ReturnFindingsBatch(items, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) Remediate(ctx context.Context, caseID, segmentID, key string, cmd RemediationCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleOrganizer); err != nil {
		return CaseView{}, err
	}
	c, _, err := s.repo.Update(ctx, caseID, cmd.ExpectedVersion, meta(key, "remediate:"+segmentID, cmd.Actor, cmd), func(c *domain.ReleaseCase) error {
		if !c.HasReturnedOpenFinding(segmentID) {
			return domain.FieldError{Field: "segmentId", Message: "仅可整改存在已退回开放发现的片段"}
		}
		return c.RemediateSegment(segmentID, cmd.ProposedText, cmd.Explanation, cmd.Actor, s.now())
	})
	if err != nil {
		return CaseView{}, err
	}
	return s.view(c), nil
}

func (s *Service) Approve(ctx context.Context, caseID, key string, cmd VersionCommand) (CaseView, error) {
	if err := requireKey(key); err != nil {
		return CaseView{}, err
	}
	if err := authorize(cmd.Actor, roleReviewer, roleReleaseManager); err != nil {
		return CaseView{}, err
	}
	return s.approveWithConfirmation(ctx, caseID, key, cmd)
}

func (s *Service) VerifyCredential(ctx context.Context, caseID string) (CaseView, error) {
	c, err := s.repo.Get(ctx, caseID)
	if err != nil {
		return CaseView{}, err
	}
	view := s.view(c)
	verification := assurance.VerifyCredentialDetailed(c)
	view.CredentialValid = &verification.Valid
	view.ValidationMessage = verification.Message
	view.CredentialVerification = &verification
	return view, nil
}
