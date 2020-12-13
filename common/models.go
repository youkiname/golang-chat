// models.go
package common

type User struct {
	Id           int64  `json: "id"`
	Username     string `json: "username"`
	PasswordHash string `json: "password_hash"`
}

type UserPublicInfo struct {
	Id       int64  `json: "id"`
	Username string `json: "username"`
}

func (user *User) GetPublicInfo() UserPublicInfo {
	return UserPublicInfo{user.Id, user.Username}
}

type Channel struct {
	Id    int64  `json: "id"`
	Title string `json: "title"`
}

// message saved to db
type SavedMessage struct {
	Id int64 `json: "id"`
	// ChatId - recipient
	// UserData.Id - sender
	ChatId    int64          `json:"chat_id"`
	UserData  UserPublicInfo `json:"user_data"`
	Text      string         `json:"text"`
	CreatedOn int64          `json:"created_on"`
}

func (msg *SavedMessage) GetChatType() string {
	return getChatType(msg.ChatId)
}

type Message struct {
	User   User   `json:"user"`
	ChatId int64  `json:"chat_id"`
	Text   string `json:"text"`
}

func (msg *Message) GetChatType() string {
	return getChatType(msg.ChatId)
}

type MessagesRequest struct {
	ChatId int64 `json:"chat_id"`
	User   User  `json:"user"`
}

type ChannelsRequest struct {
	User User `json:"user"`
}

type LoginData struct {
	Username     string `json: "username"`
	PasswordHash string `json: "password_hash"`
}

type ChatAdmins struct {
	ChatId  int64 `json: "chat_id"`
	AdminId int64 `json: "admin_id"`
}

type RegistrationError struct {
	Description string `json: "description"`
}

type LoginError struct {
	Description string `json: "description"`
}

func getChatType(chatId int64) string {
	if chatId == 0 {
		return "group"
	}
	return "private"
}
