package service

import "github.com/auto-code-os/auto-code-os/server/pkg/models"

// IssueTokensForTest exposes issueTokens for use in handler tests.
// This file is only compiled when running tests (due to _test.go convention
// being insufficient for cross-package access, so we use an export helper).
func (s *AuthService) IssueTokensForTest(user *models.User) (models.AuthTokens, error) {
	return s.issueTokens(user)
}
