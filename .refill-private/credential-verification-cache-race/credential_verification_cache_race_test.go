package credential_cache_race_test

import (
	"fmt"
	"sync"
	"testing"

	"oral-history-release-studio/internal/assurance"
	"oral-history-release-studio/internal/domain"
)

func TestConcurrentCredentialVerificationUsesSafeCache(t *testing.T) {
	segments := make([]domain.TranscriptSegment, 64)
	for i := range segments {
		segments[i] = domain.TranscriptSegment{
			ID:           fmt.Sprintf("segment-%02d", i),
			Sequence:     i + 1,
			ProposedText: fmt.Sprintf("获准公开文本-%02d", i),
			Disposition:  "保持原文",
		}
	}
	rc := &domain.ReleaseCase{
		ID:       "case-private-concurrent-verification",
		State:    domain.StateApproved,
		Version:  17,
		Segments: segments,
		Credential: &domain.ReleaseCredential{
			ID:              "credential-private",
			ApprovedVersion: 17,
		},
	}

	const readers = 16
	start := make(chan struct{})
	results := make(chan assurance.CredentialVerification, readers)
	var ready sync.WaitGroup
	var finished sync.WaitGroup
	ready.Add(readers)
	finished.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer finished.Done()
			ready.Done()
			<-start
			results <- assurance.VerifyCredentialDetailed(rc)
		}()
	}
	ready.Wait()
	close(start)
	finished.Wait()
	close(results)

	for result := range results {
		if len(result.Segments) != len(segments) {
			t.Fatalf("并发校验丢失逐段结果: got=%d want=%d", len(result.Segments), len(segments))
		}
	}
}
