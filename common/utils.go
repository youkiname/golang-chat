// utils.go
package common

import (
	"time"
)

const GROUP_CHAT_ID int64 = 0

func GetTimestampNow() int64 {
	return time.Now().Unix()
}

func IsError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
