// utils.go
package utils

import (
	"time"
)

const GROUP_CHAT_ID int64 = 0
const COMMON_SECRET_KEY string = "HCT4yhsyz24iMCQsZDKV"

func GetTimestampNow() int64 {
	return time.Now().Unix()
}

func IsError(err error) bool {
	if err != nil {
		return true
	}
	return false
}
