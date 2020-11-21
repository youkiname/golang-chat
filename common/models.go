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

type SavedMessage struct {
	Id        int64          `json: "id"`
	UserData  UserPublicInfo `json:"user_data"`
	ChatId    int64          `json:"chat_id"`
	Text      string         `json:"text"`
	CreatedOn int64          `json:"created_on"`
}

func (msg *SavedMessage) GetChatType() string {
	if msg.ChatId == 0 {
		return "group"
	}
	return "private"
}

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
