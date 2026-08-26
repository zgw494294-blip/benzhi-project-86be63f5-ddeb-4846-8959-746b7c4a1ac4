package httpui_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/httpui"
	"oral-history-release-studio/internal/persistence"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(httpui.New(application.NewService(store)).Handler())
}

func TestWorkbenchAndStrictJSONAPI(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "<body>") || !strings.Contains(string(body), "公开授权工作台") {
		t.Fatalf("首页响应无效: %d", resp.StatusCode)
	}
	bad := []byte(`{"title":"案卷","intervieweeRef":"I","intendedUse":"教育","actor":"organizer:甲","unknown":1}`)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/cases", bytes.NewReader(bad))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "bad-json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("未知字段状态码=%d", resp.StatusCode)
	}
}

func TestConflictReturnsCurrentVersion(t *testing.T) {
	ts := testServer(t)
	defer ts.Close()
	payload := application.CreateCaseCommand{Title: "案卷", IntervieweeRef: "I", IntendedUse: "教育", Actor: "organizer:甲"}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/cases", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var view application.CaseView
	if err := json.NewDecoder(resp.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	segment := application.SegmentCommand{StartMillis: 0, EndMillis: 10, SourceText: "内容", Actor: "organizer:甲", ExpectedVersion: 99}
	b, _ = json.Marshal(segment)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/cases/"+view.Case.ID+"/segments", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "stale")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		CurrentVersion int64 `json:"currentVersion"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode != http.StatusConflict || result.CurrentVersion != 1 {
		t.Fatalf("冲突响应=%d version=%d", resp.StatusCode, result.CurrentVersion)
	}
}
