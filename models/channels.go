// channels.go
package models

type Channel struct {
	Id    int64  `json: "id"`
	Title string `json: "title"`
}

type ChannelsPack struct {
	Channels []Channel `json:"channels"`
}

type ChannelsRequest struct {
	User User `json:"user"`
}
