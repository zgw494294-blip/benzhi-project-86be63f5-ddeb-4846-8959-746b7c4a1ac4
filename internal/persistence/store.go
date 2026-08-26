package persistence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
)

type Store struct {
	mu           sync.Mutex
	dir          string
	snapshotPath string
	auditPath    string
	data         snapshot
}

func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("数据目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "snapshot.json")
	data, err := loadSnapshot(path)
	if err != nil {
		return nil, err
	}
	s := &Store{dir: dir, snapshotPath: path, auditPath: filepath.Join(dir, "audit.jsonl"), data: data}
	if err := recoverAuditLog(s.auditPath, data.Audit); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Create(ctx context.Context, candidate *domain.ReleaseCase, meta application.MutationMeta) (*domain.ReleaseCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if old, ok := s.data.Idempotency[meta.Key]; ok {
		if old.Fingerprint != meta.Fingerprint {
			return nil, false, domain.ErrIdempotency
		}
		c, err := cloneCase(old.Result)
		return c, true, err
	}
	if _, exists := s.data.Cases[candidate.ID]; exists {
		return nil, false, fmt.Errorf("%w: 重复案卷 ID", domain.ErrConflict)
	}
	next, err := cloneSnapshot(s.data)
	if err != nil {
		return nil, false, err
	}
	c, err := cloneCase(candidate)
	if err != nil {
		return nil, false, err
	}
	next.Cases[c.ID] = c
	e, err := nextAudit(next.Audit, c.ID, c.Version, meta.Action, meta.Actor)
	if err != nil {
		return nil, false, err
	}
	next.Audit = append(next.Audit, e)
	result, _ := cloneCase(c)
	next.Idempotency[meta.Key] = idempotencyRecord{Fingerprint: meta.Fingerprint, CaseID: c.ID, Result: result}
	if err := s.commit(next); err != nil {
		return nil, false, err
	}
	cloned, err := cloneCase(c)
	return cloned, false, err
}

func (s *Store) Get(ctx context.Context, id string) (*domain.ReleaseCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, ok := s.data.Cases[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneCase(c)
}

func (s *Store) List(ctx context.Context) ([]*domain.ReleaseCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(s.data.Cases))
	for id := range s.data.Cases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*domain.ReleaseCase, 0, len(ids))
	for _, id := range ids {
		c, err := cloneCase(s.data.Cases[id])
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) Update(ctx context.Context, id string, expected int64, meta application.MutationMeta, mutate func(*domain.ReleaseCase) error) (*domain.ReleaseCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if old, ok := s.data.Idempotency[meta.Key]; ok {
		if old.Fingerprint != meta.Fingerprint || old.CaseID != id {
			return nil, false, domain.ErrIdempotency
		}
		c, err := cloneCase(old.Result)
		return c, true, err
	}
	current, ok := s.data.Cases[id]
	if !ok {
		return nil, false, domain.ErrNotFound
	}
	if current.Version != expected {
		return nil, false, fmt.Errorf("%w: 当前 version=%d", domain.ErrConflict, current.Version)
	}
	next, err := cloneSnapshot(s.data)
	if err != nil {
		return nil, false, err
	}
	c, err := cloneCase(current)
	if err != nil {
		return nil, false, err
	}
	if err := mutate(c); err != nil {
		return nil, false, err
	}
	if c.Version <= current.Version {
		return nil, false, errors.New("领域变更未递增 version")
	}
	next.Cases[id] = c
	e, err := nextAudit(next.Audit, id, c.Version, meta.Action, meta.Actor)
	if err != nil {
		return nil, false, err
	}
	next.Audit = append(next.Audit, e)
	result, _ := cloneCase(c)
	next.Idempotency[meta.Key] = idempotencyRecord{Fingerprint: meta.Fingerprint, CaseID: id, Result: result}
	if err := s.commit(next); err != nil {
		return nil, false, err
	}
	cloned, err := cloneCase(c)
	return cloned, false, err
}

func (s *Store) SaveApprovalConfirmation(ctx context.Context, confirmation domain.ApprovalConfirmation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, exists := s.data.ApprovalConfirmations[confirmation.Token]; exists {
		return fmt.Errorf("%w: 重复确认令牌", domain.ErrConflict)
	}
	next, err := cloneSnapshot(s.data)
	if err != nil {
		return err
	}
	next.ApprovalConfirmations[confirmation.Token] = confirmation
	return s.commit(next)
}

func (s *Store) UpdateWithApprovalConfirmation(ctx context.Context, id string, expected int64, meta application.MutationMeta, claimed domain.ApprovalConfirmation, now time.Time, mutate func(*domain.ReleaseCase) error) (*domain.ReleaseCase, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if old, ok := s.data.Idempotency[meta.Key]; ok {
		if old.Fingerprint != meta.Fingerprint || old.CaseID != id {
			return nil, false, domain.ErrIdempotency
		}
		c, err := cloneCase(old.Result)
		return c, true, err
	}
	stored, ok := s.data.ApprovalConfirmations[claimed.Token]
	if !ok {
		return nil, false, domain.ErrTokenNotFound
	}
	if stored.UsedAt != nil {
		return nil, false, domain.ErrTokenUsed
	}
	if !now.Before(stored.ExpiresAt) {
		return nil, false, domain.ErrTokenExpired
	}
	if stored.CaseID != id || stored.ExpectedVersion != expected || stored.Actor != claimed.Actor || stored.ManifestHash != claimed.ManifestHash || stored.ConsentSnapshotHash != claimed.ConsentSnapshotHash {
		return nil, false, domain.ErrTokenMismatch
	}
	current, ok := s.data.Cases[id]
	if !ok {
		return nil, false, domain.ErrNotFound
	}
	if current.Version != expected {
		return nil, false, fmt.Errorf("%w: 当前 version=%d", domain.ErrConflict, current.Version)
	}
	next, err := cloneSnapshot(s.data)
	if err != nil {
		return nil, false, err
	}
	c, err := cloneCase(current)
	if err != nil {
		return nil, false, err
	}
	if err := mutate(c); err != nil {
		return nil, false, err
	}
	if c.Version <= current.Version {
		return nil, false, errors.New("领域变更未递增 version")
	}
	next.Cases[id] = c
	used := next.ApprovalConfirmations[claimed.Token]
	usedAt := domain.UTC(now)
	used.UsedAt = &usedAt
	next.ApprovalConfirmations[claimed.Token] = used
	e, err := nextAudit(next.Audit, id, c.Version, meta.Action, meta.Actor)
	if err != nil {
		return nil, false, err
	}
	next.Audit = append(next.Audit, e)
	result, _ := cloneCase(c)
	next.Idempotency[meta.Key] = idempotencyRecord{Fingerprint: meta.Fingerprint, CaseID: id, Result: result}
	if err := s.commit(next); err != nil {
		return nil, false, err
	}
	cloned, err := cloneCase(c)
	return cloned, false, err
}

func (s *Store) commit(next snapshot) error {
	oldAuditLen := len(s.data.Audit)
	if err := writeSnapshot(s.snapshotPath, next); err != nil {
		return err
	}
	if err := appendAuditLog(s.auditPath, next.Audit[oldAuditLen:]); err != nil {
		s.data = next
		return fmt.Errorf("快照已提交但审计镜像待恢复: %w", err)
	}
	s.data = next
	return nil
}
