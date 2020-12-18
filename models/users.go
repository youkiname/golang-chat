// users.go
package models

import (
	"github.com/satori/go.uuid"
)

type User struct {
	Id       int64  `json: "id"`
	Username string `json: "username"`
}

type ConnectedUser struct {
	Id        int64     `json: "id"`
	Username  string    `json: "username"`
	SecretKey uuid.UUID `json: "secret_key"` // Encrypt/Decrypt key
}
