// server.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

const FAILED_LOGIN_LIMIT int = 5
const FAILED_LOGIN_TIME_LIMIT int = 2 * 60 // 2 minutes

type ServerApp struct {
	Server         *gosocketio.Server
	ConnectedUsers map[string]UserPublicInfo // map: socket ID -> UserData
	DB             DatabaseAdapter
}

func getRemainedLoginAttemps(userId int64, db DatabaseAdapter) int {
	countFailedLogin := db.countFailedLogin(userId)
	if countFailedLogin == 0 {
		return FAILED_LOGIN_LIMIT
	}

	lastFailedLoginDate := db.getLastFailedLoginDate(userId)
	if lastFailedLoginDate+int64(FAILED_LOGIN_TIME_LIMIT) < getTimestampNow() {
		db.clearFailedLogin(userId)
		return FAILED_LOGIN_LIMIT
	}
	return FAILED_LOGIN_LIMIT - countFailedLogin
}

func (app *ServerApp) Init() {
	connectedUsers := make(map[string]UserPublicInfo)
	app.ConnectedUsers = connectedUsers

	db := DatabaseAdapter{}
	db.connectSqlite("app.db")
	app.DB = db

	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("Connected " + c.Id())
	})
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		log.Println("Disconnected " + c.Id())
		app.RemoveConnectedUserBySocketId(c.Id())
	})

	server.On("/login", func(c *gosocketio.Channel, data LoginData) {
		user, err := db.getUserByName(data.Username)
		if !isError(err) {
			remainedLoginAttempts := getRemainedLoginAttemps(user.Id, db)

			if remainedLoginAttempts > 0 && db.checkUserPassword(data.Username, data.PasswordHash) {
				log.Println("New login " + user.Username)
				app.AddConnectedUser(c.Id(), user.getPublicInfo())
				db.clearFailedLogin(user.Id)

				c.Join("main")
				c.Emit("/login", user)
			} else {
				if remainedLoginAttempts > 0 {
					db.addNewFailedLogin(user.Id)
					c.Emit("/failed-login", LoginError{fmt.Sprintf(
						"Password is not correct. You can try again: %d times",
						remainedLoginAttempts)})
				} else {
					c.Emit("/failed-login", LoginError{fmt.Sprintf(
						"You have tried login more than %d times. This username was blocked for 2 minutes.",
						FAILED_LOGIN_LIMIT)})
				}

			}
		} else {
			c.Emit("/failed-login", LoginError{"Username is not correct."})
		}

	})
	server.On("/register", func(c *gosocketio.Channel, data LoginData) {
		if !isValid(data.Username) {
			c.Emit("/failed-registeration",
				LoginError{"Username '" + data.Username + "' is not valid."})
		} else if db.isUserExist(data.Username) {
			c.Emit("/failed-registeration",
				LoginError{"Username " + data.Username + " already exists."})
		} else {
			user := User{Username: data.Username, PasswordHash: data.PasswordHash}
			db.addNewUser(&user)

			app.AddConnectedUser(c.Id(), user.getPublicInfo())

			c.Join("main")
			c.Emit("/login", user)
		}
	})
	server.On("/message", func(c *gosocketio.Channel, msg Message) {
		if db.checkUserPassword(msg.User.Username, msg.User.PasswordHash) {
			savedMessage := db.addNewMessage(msg)
			if msg.getChatType() == "group" {
				c.BroadcastTo("main", "/message", savedMessage)
			} else if msg.ChatId == msg.User.Id {
				// private notes. like saved messages in telegram
				c.Emit("/message", savedMessage)
			} else { // from user to user. Personal messages
				if app.TrySaveNewChannel(msg) {
					// send new channels to recipient (if he is online)
					app.EmitToUser(msg.ChatId, "/get-channels", db.getChannels(msg.ChatId))
				}
				c.Emit("/message", savedMessage)
				app.EmitToUser(msg.ChatId, "/message", savedMessage)
			}
		}
	})
	server.On("/get-messages", func(c *gosocketio.Channel, requestData MessagesRequest) {
		user := requestData.User
		chatId := requestData.ChatId
		if db.checkUserPassword(user.Username, user.PasswordHash) {
			messages := db.getMessagesFromChat(user.Id, chatId)
			c.Emit("/get-messages", messages)
		}
	})
	server.On("/get-channels", func(c *gosocketio.Channel, requestData ChannelsRequest) {
		user := requestData.User
		if !db.checkUserPassword(user.Username, user.PasswordHash) {
			return
		}
		channels := db.getChannels(user.Id)

		c.Emit("/get-channels", channels)
	})

	app.Server = server
}

func (app *ServerApp) Run(host string) {
	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", app.Server)

	log.Println("Starting server...")
	log.Panic(http.ListenAndServe(host, serveMux))
}

func (app *ServerApp) CloseDB() {
	app.DB.Close()
}

func (app *ServerApp) PrintConnectedUsers() {
	users := app.ConnectedUsers
	json_users, err := json.MarshalIndent(users, "", "    ")
	if isError(err) {
		log.Println(err)
	}
	fmt.Printf("Connected Users %d:\n", len(users))
	fmt.Println(string(json_users))
}

func (app *ServerApp) TrySaveNewChannel(msg Message) bool {
	db := app.DB
	// return true if new channel was saved
	result := false
	if !db.isChannelExist(msg.User.Id, msg.ChatId) {
		db.addNewChannel(msg.User.Id, msg.ChatId)
		result = true
	}
	// saves double row in order to receiver will too has ability
	// to check new messages in this channel
	if !db.isChannelExist(msg.ChatId, msg.User.Id) {
		db.addNewChannel(msg.ChatId, msg.User.Id)
		result = true
	}
	return result
}

func (app *ServerApp) AddConnectedUser(socketId string, userData UserPublicInfo) {
	users := app.ConnectedUsers
	users[socketId] = userData
}

func (app *ServerApp) RemoveConnectedUserBySocketId(socketId string) {
	delete(app.ConnectedUsers, socketId)
}

func (app *ServerApp) EmitToUser(userId int64, method string, args interface{}) {
	// emit if user is online. To all connected clients with this account
	for socketId, userData := range app.ConnectedUsers {
		if userData.Id == userId {
			channel, err := app.Server.GetChannel(socketId)
			if !isError(err) {
				channel.Emit(method, args)
			} else {
				log.Println(err)
			}
		}
	}
}

func isValid(username string) bool {
	for _, c := range username {
		if c == ' ' {
			return false
		}
	}
	return true
}

func main() {
	serverApp := ServerApp{}
	serverApp.Init()

	defer serverApp.CloseDB()

	serverApp.Run("localhost:3811")
}
