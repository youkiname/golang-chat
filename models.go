// models.go
package main

type User struct {
	Id           int
	Username     string
	PasswordHash string
}

type Message struct {
	User User   `json:"user"`
	Text string `json:"text"`
}

type LoginData struct {
	Username     string
	PasswordHash string
}
