package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"oral-history-release-studio/internal/application"
	"oral-history-release-studio/internal/domain"
)

type selfcheckClient struct {
	base   string
	client *http.Client
	serial int
}

func (c *selfcheckClient) request(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		c.serial++
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%02d", c.serial))
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s 返回 %d: %s", method, path, resp.StatusCode, b)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("解析 %s 响应失败: %w", path, err)
	}
	return nil
}

func runSelfcheck(ctx context.Context, listener net.Listener, server *http.Server) error {
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(closeCtx)
		<-serveDone
	}()
	checkCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	c := &selfcheckClient{base: "http://" + listener.Addr().String(), client: &http.Client{Timeout: 3 * time.Second}}
	var view application.CaseView
	create := application.CreateCaseCommand{Title: "自检口述史案卷", IntervieweeRef: "INT-SELF-001", IntendedUse: "公共教育", Actor: "organizer:自检整理员"}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases", create, &view); err != nil {
		return err
	}
	caseID := view.Case.ID
	segment := application.SegmentCommand{StartMillis: 0, EndMillis: 4200, SourceText: "受访者讲述了与家人有关的具体经历", ProposedText: "受访者讲述了与家人有关的具体经历", SensitivityTag: domain.SensitivityPersonal, Actor: "organizer:自检整理员", ExpectedVersion: view.Case.Version}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/segments", segment, &view); err != nil {
		return err
	}
	segmentID := view.Case.Segments[0].ID
	now := time.Now().UTC()
	consent := application.ConsentCommand{Scope: []string{"*"}, AllowedUses: []string{"公共教育"}, ValidFrom: now.AddDate(-1, 0, 0), ExpiresAt: now.AddDate(2, 0, 0), SignedBy: "受访者本人", SignedAt: now, Actor: "organizer:自检整理员", ExpectedVersion: view.Case.Version}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/consents", consent, &view); err != nil {
		return err
	}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/freeze", application.VersionCommand{Actor: "organizer:自检整理员", ExpectedVersion: view.Case.Version}, &view); err != nil {
		return err
	}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/checks", application.VersionCommand{Actor: "reviewer:自检复核员", ExpectedVersion: view.Case.Version}, &view); err != nil {
		return err
	}
	if view.OpenBlockers != 1 || len(view.Case.Findings) == 0 {
		return fmt.Errorf("全量检查未产生预期敏感内容阻断项")
	}
	findingID := view.Case.Findings[0].ID
	ret := application.ReturnCommand{Actor: "reviewer:自检复核员", Note: "请去除可识别家庭细节", ExpectedVersion: view.Case.Version}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/findings/"+findingID+"/return", ret, &view); err != nil {
		return err
	}
	rem := application.RemediationCommand{Actor: "organizer:自检整理员", ProposedText: "受访者讲述了一段个人经历", Explanation: "已概括改写并移除家庭细节", ExpectedVersion: view.Case.Version}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/segments/"+segmentID+"/remediation", rem, &view); err != nil {
		return err
	}
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/segments/"+segmentID+"/recheck", application.VersionCommand{Actor: "reviewer:自检复核员", ExpectedVersion: view.Case.Version}, &view); err != nil {
		return err
	}
	if view.OpenBlockers != 0 {
		return fmt.Errorf("定向复检后仍有 %d 个阻断项", view.OpenBlockers)
	}
	var findings application.FindingQueryResult
	if err := c.request(checkCtx, http.MethodGet, "/api/cases/"+caseID+"/findings?status=closed", nil, &findings); err != nil {
		return err
	}
	if findings.CaseVersion != view.Case.Version || findings.Statistics.MatchedTotal == 0 {
		return fmt.Errorf("发现分组检索未返回预期关闭项")
	}
	approveCmd := application.VersionCommand{Actor: "release_manager:自检发布负责人", ExpectedVersion: view.Case.Version}
	var preview application.ApprovalPreview
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/approval-preview", approveCmd, &preview); err != nil {
		return err
	}
	if len(preview.Segments) != 1 || preview.ConfirmationToken == "" {
		return fmt.Errorf("批准清单预览未签发有效确认令牌")
	}
	approveCmd.ConfirmationToken = preview.ConfirmationToken
	if err := c.request(checkCtx, http.MethodPost, "/api/cases/"+caseID+"/approve", approveCmd, &view); err != nil {
		return err
	}
	if err := c.request(checkCtx, http.MethodGet, "/api/cases/"+caseID+"/credential", nil, &view); err != nil {
		return err
	}
	if view.CredentialValid == nil || !*view.CredentialValid || view.Case.Credential == nil || view.CredentialVerification == nil || len(view.CredentialVerification.Segments) != 1 || view.CredentialVerification.Segments[0].Status != "passed" {
		return fmt.Errorf("公开凭据校验失败: %s", view.ValidationMessage)
	}
	return nil
}
