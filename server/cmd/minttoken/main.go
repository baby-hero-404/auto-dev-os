package main

import (
	"fmt"
	"os"

	"github.com/auto-code-os/auto-code-os/server/internal/service"
	"github.com/auto-code-os/auto-code-os/server/pkg/models"
)

func main() {
	authSvc := service.NewAuthService(nil, os.Getenv("JWT_SECRET"))
	u := &models.User{ID: "dbdd1667-92e2-4e3a-9b92-363febf58880", Email: "test@gmail.com", OrgID: "22de748e-6f38-4278-8bab-12ce2fc1b621", Role: "admin"}
	tokens, err := authSvc.IssueTokensForTest(u)
	if err != nil {
		panic(err)
	}
	fmt.Println(tokens.AccessToken)
}
