// server.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"

	"chat/common"
)

const FAILED_LOGIN_LIMIT int = 5
const FAILED_LOGIN_TIME_LIMIT int = 2 * 60 // 2 minutes

type ServerApp struct {
	Server         *gosocketio.Server
	ConnectedUsers map[string]common.UserPublicInfo // map: socket ID -> UserData
	DB             common.DatabaseAdapter
}

func getRemainedLoginAttemps(userId int64, db common.DatabaseAdapter) int {
	// returns remained attemps count.
	// And clears unsuccessful login count if it is no longer relevant

	countFailedLogin := db.CountFailedLogin(userId)
	if countFailedLogin == 0 {
		return FAILED_LOGIN_LIMIT
	}

	lastFailedLoginDate := db.GetLastFailedLoginDate(userId)
	if lastFailedLoginDate+int64(FAILED_LOGIN_TIME_LIMIT) < common.GetTimestampNow() {
		db.ClearFailedLogin(userId)
		return FAILED_LOGIN_LIMIT
	}
	return FAILED_LOGIN_LIMIT - countFailedLogin
}

func (app *ServerApp) Init() {
	// connects to db. Sets callbacks to socket.io server
	connectedUsers := make(map[string]common.UserPublicInfo)
	app.ConnectedUsers = connectedUsers

	db := common.DatabaseAdapter{}
	db.ConnectSqlite("app.db")
	app.DB = db

	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("Connected " + c.Id())
	})
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		log.Println("Disconnected " + c.Id())
		app.RemoveConnectedUser(c.Id())
	})

	server.On("/login", app.processNewLogin)
	server.On("/register", app.processNewRegistration)
	server.On("/message", app.processNewMessage)
	server.On("/get-messages", app.processMessagesRequest)
	server.On("/get-channels", app.processChannelsRequest)

	app.Server = server
}

func (app *ServerApp) Run(host string) {
	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", app.Server)

	log.Println("Starting server at " + host)
	log.Panic(http.ListenAndServe(host, serveMux))
}

func (app *ServerApp) CloseDB() {
	app.DB.Close()
}

func (app *ServerApp) processNewLogin(c *gosocketio.Channel, data common.LoginData) {
	user, err := app.DB.GetUserByName(data.Username)
	isUsernameValid := !common.IsError(err)

	remainedLoginAttempts := getRemainedLoginAttemps(user.Id, app.DB)
	isPasswordCorrect := app.DB.CheckUserPassword(data.Username, data.PasswordHash)

	isLoginSuccessful := isPasswordCorrect && isUsernameValid &&
		remainedLoginAttempts > 0

	if isLoginSuccessful {
		app.processSuccessfulLogin(c, user)
	} else {
		app.processUnsuccessfulLogin(c, user, isUsernameValid,
			remainedLoginAttempts)
	}
}

func (app *ServerApp) processSuccessfulLogin(c *gosocketio.Channel, user common.User) {
	log.Println("New login " + user.Username)
	app.AddConnectedUser(c.Id(), user.GetPublicInfo())
	app.DB.ClearFailedLogin(user.Id)

	c.Join("main")
	c.Emit("/login", user)
}

func (app *ServerApp) processUnsuccessfulLogin(c *gosocketio.Channel,
	user common.User, isUsernameValid bool, remainedLoginAttempts int) {
	if !isUsernameValid {
		c.Emit("/failed-login", common.LoginError{"Username is not correct."})
	} else if remainedLoginAttempts > 0 {
		app.DB.AddNewFailedLogin(user.Id)
		c.Emit("/failed-login", common.LoginError{fmt.Sprintf(
			"Password is not correct. You can try again: %d times",
			remainedLoginAttempts)})
	} else {
		c.Emit("/failed-login", common.LoginError{fmt.Sprintf(
			"You have tried login more than %d times. This username was blocked for 2 minutes.",
			FAILED_LOGIN_LIMIT)})
	}
}

func (app *ServerApp) processNewRegistration(c *gosocketio.Channel, data common.LoginData) {
	if !isValid(data.Username) {
		c.Emit("/failed-registeration",
			common.LoginError{"Username '" + data.Username + "' is not valid."})
	} else if app.DB.IsUserExist(data.Username) {
		c.Emit("/failed-registeration",
			common.LoginError{"Username " + data.Username + " already exists."})
	} else {
		user := common.User{Username: data.Username, PasswordHash: data.PasswordHash}
		app.DB.AddNewUser(&user)

		app.AddConnectedUser(c.Id(), user.GetPublicInfo())

		c.Join("main")
		c.Emit("/login", user)
	}
}

func (app *ServerApp) processNewMessage(c *gosocketio.Channel, msg common.Message) {
	if app.DB.CheckUserPassword(msg.User.Username, msg.User.PasswordHash) {
		savedMessage := app.DB.AddNewMessage(msg)
		if msg.GetChatType() == "group" {
			c.BroadcastTo("main", "/message", savedMessage)
		} else if msg.ChatId == msg.User.Id {
			// private notes. like saved messages in telegram.
			c.Emit("/message", savedMessage)
		} else { // from user to user. Personal messages.
			if app.TrySaveNewChannel(msg) {
				// send new channels to recipient (if he is online).
				app.EmitToUser(msg.ChatId, "/get-channels", app.DB.GetChannels(msg.ChatId))
			}
			c.Emit("/message", savedMessage)
			app.EmitToUser(msg.ChatId, "/message", savedMessage)
		}
	}
}

func (app *ServerApp) processMessagesRequest(c *gosocketio.Channel, requestData common.MessagesRequest) {
	user := requestData.User
	chatId := requestData.ChatId
	if app.DB.CheckUserPassword(user.Username, user.PasswordHash) {
		c.Emit("/get-messages", app.DB.GetMessagesFromChat(user.Id, chatId))
	}
}

func (app *ServerApp) processChannelsRequest(c *gosocketio.Channel, requestData common.ChannelsRequest) {
	user := requestData.User
	if !app.DB.CheckUserPassword(user.Username, user.PasswordHash) {
		return
	}
	c.Emit("/get-channels", app.DB.GetChannels(user.Id))
}

func (app *ServerApp) PrintConnectedUsers() {
	users := app.ConnectedUsers
	json_users, err := json.MarshalIndent(users, "", "    ")
	if common.IsError(err) {
		log.Println(err)
	}
	fmt.Printf("Connected Users %d:\n", len(users))
	fmt.Println(string(json_users))
}

func (app *ServerApp) TrySaveNewChannel(msg common.Message) bool {
	// return true if new channel was saved
	db := app.DB
	result := false
	if !db.IsChannelExist(msg.User.Id, msg.ChatId) {
		db.AddNewChannel(msg.User.Id, msg.ChatId)
		result = true
	}
	// saves double row in order to receiver will too has ability
	// to check new messages in this channel
	if !db.IsChannelExist(msg.ChatId, msg.User.Id) {
		db.AddNewChannel(msg.ChatId, msg.User.Id)
		result = true
	}
	return result
}

func (app *ServerApp) AddConnectedUser(socketId string, userData common.UserPublicInfo) {
	users := app.ConnectedUsers
	users[socketId] = userData
}

func (app *ServerApp) RemoveConnectedUser(socketId string) {
	delete(app.ConnectedUsers, socketId)
}

func (app *ServerApp) EmitToUser(userId int64, method string, args interface{}) {
	// emit if user is online. To all connected clients with this account
	for socketId, userData := range app.ConnectedUsers {
		if userData.Id == userId {
			channel, err := app.Server.GetChannel(socketId)
			if !common.IsError(err) {
				channel.Emit(method, args)
			} else {
				log.Println(err)
			}
		}
	}
}

func isValid(username string) bool {
	// username must not have spaces
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

	host, port := common.GetHostDataFromSettingsFile()

	serverApp.Run(fmt.Sprintf("%s:%d", host, port))
}
