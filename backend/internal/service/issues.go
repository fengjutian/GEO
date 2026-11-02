package service

import (
	"context"
	"errors"
	"strings"

	"github.com/fengjutian/GEO/backend/internal/domain"
	"github.com/fengjutian/GEO/backend/internal/store"
)

func ValidIssueStatus(status string) bool {
	switch status {
	case domain.IssueStatusOpen, domain.IssueStatusInProgress, domain.IssueStatusResolved, domain.IssueStatusIgnored:
		return true
	default:
		return false
	}
}

func (s *Service) UpdateIssue(ctx context.Context, id uint, status, note string) (domain.Finding, error) {
	status = strings.TrimSpace(status)
	note = strings.TrimSpace(note)
	if !ValidIssueStatus(status) {
		return domain.Finding{}, validation("状态必须为 open、in_progress、resolved 或 ignored")
	}
	if len([]rune(note)) > 1000 {
		return domain.Finding{}, validation("备注不能超过 1000 个字符")
	}
	finding, err := s.store.GetFinding(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return domain.Finding{}, notFound("问题记录不存在")
		}
		return domain.Finding{}, internal("无法读取问题记录", err)
	}
	if finding.Status == status {
		return finding, nil
	}
	updated, err := s.store.UpdateIssueStatus(ctx, finding, status, note)
	if err != nil {
		return domain.Finding{}, internal("更新问题状态失败", err)
	}
	return updated, nil
}
