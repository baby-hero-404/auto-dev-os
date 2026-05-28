package models

import "time"

const (
	UserRoleAdmin  = "admin"
	UserRoleMember = "member"
)

type User struct {
	ID           string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	Email        string    `json:"email" gorm:"not null;unique"`
	PasswordHash string    `json:"-" gorm:"not null"`
	OrgID        string    `json:"org_id" gorm:"type:uuid;not null"`
	Role         string    `json:"role" gorm:"default:'admin'"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OrgName  string `json:"org_name"`
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthResponse struct {
	User   *User      `json:"user"`
	Tokens AuthTokens `json:"tokens"`
}
