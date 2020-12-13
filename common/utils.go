// utils.go
package common

import (
	"encoding/hex"
	"time"

	"crypto/sha256"
)

const GROUP_CHAT_ID int64 = 0

func GetPasswordHash(password string) string {
	hashedPassword := password
	for i := 0; i < 10; i++ {
		hash := sha256.Sum256([]byte(hashedPassword))
		hashedPassword = hex.EncodeToString(hash[:])
	}
	return hashedPassword
}

func encryptMessage() {

}

func decryptMessage() {

}

func GetTimestampNow() int64 {
	return time.Now().Unix()
}

func IsError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
