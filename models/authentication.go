// authentication.go
package models

import (
	"github.com/satori/go.uuid"
)

type Session struct {
	User      User      `json:"user"`
	SecretKey uuid.UUID `json:"secret_key"`
}

type AuthRequest struct {
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}

type SuccessfulAuth struct {
	User      User      `json: "user"`
	SecretKey uuid.UUID `json: "secret_key"`
	CommonKey uuid.UUID `json: "common_key"`
}

type AuthError struct {
	Description string `json: "description"`
	Process     string `json: "process"` // registration or login
}
