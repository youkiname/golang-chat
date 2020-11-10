// utils.go
package main

import (
	"crypto/md5"
	"encoding/hex"
)

func GetMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

func isError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
