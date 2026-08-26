package persistence

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"oral-history-release-studio/internal/domain"
)

func loadSnapshot(path string) (snapshot, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshot{SchemaVersion: schemaVersion, Cases: map[string]*domain.ReleaseCase{}, Idempotency: map[string]idempotencyRecord{}, Audit: []auditEntry{}, ApprovalConfirmations: map[string]domain.ApprovalConfirmation{}}, nil
	}
	if err != nil {
		return snapshot{}, err
	}
	var data snapshot
	if err := json.Unmarshal(b, &data); err != nil {
		return snapshot{}, fmt.Errorf("读取快照失败: %w", err)
	}
	if data.SchemaVersion != schemaVersion {
		return snapshot{}, fmt.Errorf("不支持的 schemaVersion: %d", data.SchemaVersion)
	}
	if data.Cases == nil {
		data.Cases = make(map[string]*domain.ReleaseCase)
	}
	if data.Idempotency == nil {
		data.Idempotency = make(map[string]idempotencyRecord)
	}
	if data.ApprovalConfirmations == nil {
		data.ApprovalConfirmations = make(map[string]domain.ApprovalConfirmation)
	}
	if err := validateAudit(data.Audit); err != nil {
		return snapshot{}, err
	}
	return data, nil
}

func writeSnapshot(path string, data snapshot) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	clean := true
	defer func() {
		if clean {
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	clean = false
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func readAuditLog(path string) ([]auditEntry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var entries []auditEntry
	for scanner.Scan() {
		var e auditEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("审计日志格式错误: %w", err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, validateAudit(entries)
}

func appendAuditLog(path string, entries []auditEntry) error {
	if len(entries) == 0 {
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			_ = f.Close()
			return err
		}
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func recoverAuditLog(path string, expected []auditEntry) error {
	actual, err := readAuditLog(path)
	if err != nil {
		return err
	}
	if len(actual) > len(expected) {
		return fmt.Errorf("审计日志领先于快照")
	}
	for i := range actual {
		if actual[i].Hash != expected[i].Hash {
			return fmt.Errorf("审计日志与快照在序号 %d 不一致", i+1)
		}
	}
	return appendAuditLog(path, expected[len(actual):])
}
