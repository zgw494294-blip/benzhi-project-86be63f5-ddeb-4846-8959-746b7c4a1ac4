package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type auditPayload struct {
	Sequence     int64     `json:"sequence"`
	CaseID       string    `json:"caseId"`
	CaseVersion  int64     `json:"caseVersion"`
	Action       string    `json:"action"`
	Actor        string    `json:"actor"`
	At           time.Time `json:"at"`
	PreviousHash string    `json:"previousHash"`
}

func nextAudit(entries []auditEntry, caseID string, version int64, action, actor string) (auditEntry, error) {
	previous := ""
	if len(entries) > 0 {
		previous = entries[len(entries)-1].Hash
	}
	p := auditPayload{Sequence: int64(len(entries) + 1), CaseID: caseID, CaseVersion: version, Action: action, Actor: actor, At: time.Now().UTC().Truncate(time.Millisecond), PreviousHash: previous}
	b, err := json.Marshal(p)
	if err != nil {
		return auditEntry{}, err
	}
	sum := sha256.Sum256(b)
	return auditEntry{Sequence: p.Sequence, CaseID: p.CaseID, CaseVersion: p.CaseVersion, Action: p.Action, Actor: p.Actor, At: p.At, PreviousHash: p.PreviousHash, Hash: hex.EncodeToString(sum[:])}, nil
}

func validateAudit(entries []auditEntry) error {
	previous := ""
	for i, e := range entries {
		if e.Sequence != int64(i+1) {
			return fmt.Errorf("审计序号不连续：%d", e.Sequence)
		}
		if e.PreviousHash != previous {
			return fmt.Errorf("审计摘要链在序号 %d 断裂", e.Sequence)
		}
		p := auditPayload{Sequence: e.Sequence, CaseID: e.CaseID, CaseVersion: e.CaseVersion, Action: e.Action, Actor: e.Actor, At: e.At, PreviousHash: e.PreviousHash}
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		if e.Hash != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("审计摘要在序号 %d 无效", e.Sequence)
		}
		previous = e.Hash
	}
	return nil
}
