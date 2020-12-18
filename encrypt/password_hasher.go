// password_hasher.go
package encrypt

import (
	"encoding/hex"

	"crypto/sha256"
)

func GetPasswordHash(password string) string {
	hashedPassword := password
	for i := 0; i < 10; i++ {
		hash := sha256.Sum256([]byte(hashedPassword))
		hashedPassword = hex.EncodeToString(hash[:])
	}
	return hashedPassword
}
