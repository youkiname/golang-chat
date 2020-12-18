// messages.go
package models

import (
	"github.com/satori/go.uuid"
)

type Message struct {
	User   User   `json:"user"`
	ChatId int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (msg *Message) GetChatType() string {
	if msg.ChatId == 0 {
		return "group"
	}
	return "private"
}

// message saved to db
type SavedMessage struct {
	Message
	Id        int64 `json: "id"`
	CreatedOn int64 `json:"created_on"`
}

type SavedMessagesPack struct {
	Messages []SavedMessage `json:"messages"`
}

type MessagesRequest struct {
	ChatId int64 `json:"chat_id"`
	User   User  `json:"user"`
}

type MessagesSavingRequest struct {
	Message   Message   `json:"message"`
	SecretKey uuid.UUID `json:"secret_key"`
}
