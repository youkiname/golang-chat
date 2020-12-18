// interfaces.go
package models

import "github.com/satori/go.uuid"

type Encryptable interface {
	EncryptWith(key uuid.UUID) string
}
