// utils.go
package main

import (
	"encoding/hex"
	"time"

	"crypto/sha256"
)

const GROUP_CHAT_ID int64 = 0

func getPasswordHash(password string) string {
	hashedPassword := password
	for i := 0; i < 10; i++ {
		hash := sha256.Sum256([]byte(hashedPassword))
		hashedPassword = hex.EncodeToString(hash[:])
	}
	return hashedPassword
}

func getTimestampNow() int64 {
	return time.Now().Unix()
}

func isError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
