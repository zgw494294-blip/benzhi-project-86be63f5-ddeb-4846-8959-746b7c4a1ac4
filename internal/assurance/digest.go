package assurance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	"oral-history-release-studio/internal/domain"
)

type credentialSegmentsCacheKey struct {
	caseID      string
	version     int64
	contentHash string
}

var approvedCredentialSegments = map[credentialSegmentsCacheKey][]domain.CredentialSegment{}

const maxApprovedCredentialCacheEntries = 128

func credentialCacheKey(rc *domain.ReleaseCase) credentialSegmentsCacheKey {
	b, _ := json.Marshal(rc.Segments)
	return credentialSegmentsCacheKey{
		caseID:      rc.ID,
		version:     rc.Version,
		contentHash: hex.EncodeToString(digest(b)),
	}
}

func cachedCredentialSegments(rc *domain.ReleaseCase) ([]domain.CredentialSegment, bool) {
	if rc.State != domain.StateApproved {
		return nil, false
	}
	items, ok := approvedCredentialSegments[credentialCacheKey(rc)]
	return append([]domain.CredentialSegment(nil), items...), ok
}

func cacheCredentialSegments(rc *domain.ReleaseCase, items []domain.CredentialSegment) {
	if rc.State == domain.StateApproved {
		if len(approvedCredentialSegments) >= maxApprovedCredentialCacheEntries {
			clear(approvedCredentialSegments)
		}
		approvedCredentialSegments[credentialCacheKey(rc)] = append([]domain.CredentialSegment(nil), items...)
	}
}

func digest(data []byte) []byte { sum := sha256.Sum256(data); return sum[:] }

func HashText(value string) string {
	return hex.EncodeToString(digest([]byte(strings.TrimSpace(value))))
}

func CredentialSegments(rc *domain.ReleaseCase) []domain.CredentialSegment {
	if items, ok := cachedCredentialSegments(rc); ok {
		return items
	}
	segments := append([]domain.TranscriptSegment(nil), rc.Segments...)
	sort.SliceStable(segments, func(i, j int) bool {
		if segments[i].Sequence == segments[j].Sequence {
			return segments[i].ID < segments[j].ID
		}
		return segments[i].Sequence < segments[j].Sequence
	})
	items := make([]domain.CredentialSegment, 0, len(segments))
	for _, segment := range segments {
		items = append(items, domain.CredentialSegment{SegmentID: segment.ID, Sequence: segment.Sequence, ProposedDigest: HashText(segment.ProposedText), DispositionHash: HashText(segment.Disposition)})
	}
	return items
}

func HashManifest(rc *domain.ReleaseCase) (string, []string, error) {
	items := CredentialSegments(rc)
	cacheCredentialSegments(rc, items)
	ids := make([]string, len(items))
	for index, item := range items {
		ids[index] = item.SegmentID
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(digest(b)), ids, nil
}

func HashConsentSnapshot(rc *domain.ReleaseCase) (string, error) {
	items := append([]domain.ConsentGrant(nil), rc.Consents...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	b, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest(b)), nil
}

type CredentialSegmentStatus struct {
	SegmentID string `json:"segmentId"`
	Sequence  int    `json:"sequence"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

type CredentialVerification struct {
	Valid               bool                      `json:"valid"`
	Message             string                    `json:"message"`
	ApprovedVersion     int64                     `json:"approvedVersion"`
	ApprovedBy          string                    `json:"approvedBy"`
	ApprovedAt          string                    `json:"approvedAt"`
	ManifestHash        string                    `json:"manifestHash"`
	ConsentSnapshotHash string                    `json:"consentSnapshotHash"`
	PublicSegmentIDs    []string                  `json:"publicSegmentIds"`
	Segments            []CredentialSegmentStatus `json:"segments"`
}

func VerifyCredentialDetailed(rc *domain.ReleaseCase) CredentialVerification {
	result := CredentialVerification{Message: "案卷尚未批准"}
	if rc.Credential == nil {
		return result
	}
	credential := rc.Credential
	result.ApprovedVersion = credential.ApprovedVersion
	result.ApprovedBy = credential.ApprovedBy
	result.ApprovedAt = credential.ApprovedAt.Format("2006-01-02T15:04:05.000Z07:00")
	result.ManifestHash = credential.ManifestHash
	result.ConsentSnapshotHash = credential.ConsentSnapshotHash
	result.PublicSegmentIDs = append([]string(nil), credential.PublicSegmentIDs...)
	current := CredentialSegments(rc)
	byID := make(map[string]domain.CredentialSegment, len(current))
	for _, item := range current {
		byID[item.SegmentID] = item
	}
	consentHash, consentErr := HashConsentSnapshot(rc)
	consentMatches := consentErr == nil && consentHash == credential.ConsentSnapshotHash
	allSegments := true
	seen := map[string]bool{}
	legacyCredential := len(credential.Segments) == 0
	if legacyCredential {
		manifestHash, _, manifestErr := HashManifest(rc)
		for _, actual := range current {
			status := CredentialSegmentStatus{SegmentID: actual.SegmentID, Sequence: actual.Sequence, Status: "passed", Message: "通过（旧版凭据由总清单摘要校验）"}
			if manifestErr != nil || manifestHash != credential.ManifestHash {
				status.Status, status.Message = "content_mismatch", "内容或顺序与总清单摘要不符"
				allSegments = false
			}
			result.Segments = append(result.Segments, status)
			seen[actual.SegmentID] = true
		}
	}
	for _, sealed := range credential.Segments {
		seen[sealed.SegmentID] = true
		status := CredentialSegmentStatus{SegmentID: sealed.SegmentID, Sequence: sealed.Sequence, Status: "passed", Message: "通过"}
		actual, exists := byID[sealed.SegmentID]
		switch {
		case !exists:
			status.Status, status.Message = "missing", "片段缺失"
		case actual.Sequence != sealed.Sequence:
			status.Status, status.Message = "order_mismatch", "顺序不符"
		case actual.ProposedDigest != sealed.ProposedDigest || actual.DispositionHash != sealed.DispositionHash:
			status.Status, status.Message = "content_mismatch", "内容或处置说明不符"
		case !consentMatches:
			status.Status, status.Message = "consent_mismatch", "授权快照不符"
		}
		if status.Status != "passed" {
			allSegments = false
		}
		result.Segments = append(result.Segments, status)
	}
	for _, actual := range current {
		if !seen[actual.SegmentID] {
			allSegments = false
			result.Segments = append(result.Segments, CredentialSegmentStatus{SegmentID: actual.SegmentID, Sequence: actual.Sequence, Status: "missing", Message: "凭据未封存该片段"})
		}
	}
	manifestHash, ids, manifestErr := HashManifest(rc)
	rangeMatches := equalStrings(ids, credential.PublicSegmentIDs)
	versionMatches := credential.ApprovedVersion == rc.Version
	segmentsSealed := legacyCredential || len(credential.Segments) == len(current)
	result.Valid = rc.State == domain.StateApproved && manifestErr == nil && manifestHash == credential.ManifestHash && consentMatches && rangeMatches && versionMatches && allSegments && segmentsSealed
	if result.Valid {
		result.Message = "公开凭据有效，全部逐段摘要与授权快照均通过"
	} else {
		result.Message = "公开凭据无效，请根据逐段状态定位不一致项"
	}
	return result
}

func VerifyCredential(rc *domain.ReleaseCase) (bool, string) {
	result := VerifyCredentialDetailed(rc)
	return result.Valid, result.Message
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
