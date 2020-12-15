// server.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/satori/go.uuid"

	"chat/common"
	"chat/db"
	"chat/models"
)

const FAILED_LOGIN_LIMIT int = 5
const FAILED_LOGIN_TIME_LIMIT int = 2 * 60 // 2 minutes

type ServerApp struct {
	Server    *gosocketio.Server
	Sessions  map[string]models.Session // map: socket ID -> Session data
	CommonKey uuid.UUID                 // common key for group channel
	DB        db.DatabaseAdapter
}

func getRemainedLoginAttemps(userId int64, db db.DatabaseAdapter) int {
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
	// Connects to db. Generate common key (for group channel)
	// And sets callbacks to socket.io server
	app.Sessions = make(map[string]models.Session)

	db := db.DatabaseAdapter{}
	db.ConnectSqlite("app.db")
	app.DB = db

	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, app.processConnection)
	server.On(gosocketio.OnDisconnection, app.processDisconnection)

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

func (app *ServerApp) processConnection(c *gosocketio.Channel) {
	log.Println("Connected " + c.Id())
}

func (app *ServerApp) processDisconnection(c *gosocketio.Channel) {
	log.Println("Disconnected " + c.Id())
	app.removeSession(c.Id())
}

func (app *ServerApp) processNewLogin(c *gosocketio.Channel, authData models.AuthRequest) {
	user, err := app.DB.GetUserByName(authData.Username)
	isUsernameValid := !common.IsError(err)

	remainedLoginAttempts := getRemainedLoginAttemps(user.Id, app.DB)
	isPasswordCorrect := app.DB.CheckUserPassword(
		authData.Username, authData.PasswordHash)

	isLoginSuccessful := isPasswordCorrect && isUsernameValid &&
		remainedLoginAttempts > 0

	if isLoginSuccessful {
		app.processSuccessfulLogin(c, user)
	} else {
		app.processUnsuccessfulLogin(c, user, isUsernameValid,
			remainedLoginAttempts)
	}
}

func (app *ServerApp) processSuccessfulLogin(c *gosocketio.Channel, user models.User) {
	log.Println("New login " + user.Username)
	newSession := app.createSession(c.Id(), user)

	app.DB.ClearFailedLogin(user.Id)
	app.PrintSessions()
	c.Join("main")
	c.Emit("/login", models.SuccessfulAuth{
		User:      user,
		SecretKey: newSession.SecretKey})
}

func (app *ServerApp) processUnsuccessfulLogin(c *gosocketio.Channel,
	user models.User, isUsernameValid bool, remainedLoginAttempts int) {
	errorDescription := ""
	if !isUsernameValid {
		errorDescription = "Username is not correct."
	} else if remainedLoginAttempts > 0 {
		app.DB.AddNewFailedLogin(user.Id)
		errorDescription = fmt.Sprintf(
			"Password is not correct. You can try again: %d times",
			remainedLoginAttempts)
	} else {
		errorDescription = fmt.Sprintf(
			"You have tried login more than %d times. This username was blocked for 2 minutes.",
			FAILED_LOGIN_LIMIT)
	}
	c.Emit("/failed-login", models.AuthError{errorDescription, "login"})
}

func (app *ServerApp) processNewRegistration(c *gosocketio.Channel, authData models.AuthRequest) {
	authError := models.AuthError{Description: "", Process: "registration"}
	if !isValid(authData.Username) {
		authError.Description = "Username " + authData.Username + " is not valid."
		c.Emit("/failed-registeration", authError)
	} else if app.DB.IsUserExist(authData.Username) {
		authError.Description = "Username " + authData.Username + " already exists."
		c.Emit("/failed-registeration", authError)
	} else {
		user := models.User{Username: authData.Username}
		app.DB.AddNewUser(&user, authData.PasswordHash)
		app.processSuccessfulLogin(c, user)
	}
}

func (app *ServerApp) processNewMessage(c *gosocketio.Channel, msg models.Message) {
	secretKey, err := app.getClientSecretKey(c.Id(), msg.User)
	if common.IsError(err) {
		return
	}
	savedMessage := app.DB.AddNewMessage(msg)
	if msg.GetChatType() == "group" {
		app.EmitToAll("/message", savedMessage)
		return
	}
	if msg.ChatId != msg.User.Id { // Personal message
		if app.trySaveNewChannel(msg) {
			// send new channels to recipient (if he is online).
			pack := models.ChannelsPack{app.DB.GetChannels(msg.ChatId)}
			app.EmitToUser(msg.ChatId, "/get-channels", pack)
		}
		app.EmitToUser(msg.ChatId, "/message", savedMessage)
	}
	c.Emit("/message", savedMessage.EncryptWith(secretKey))
}

func (app *ServerApp) processMessagesRequest(c *gosocketio.Channel,
	requestData models.MessagesRequest) {
	secretKey, err := app.getClientSecretKey(c.Id(), requestData.User)
	if common.IsError(err) {
		return
	}
	user := requestData.User
	chatId := requestData.ChatId
	messages := app.DB.GetMessagesFromChat(user.Id, chatId)
	pack := models.SavedMessagesPack{messages}
	c.Emit("/get-messages", pack.EncryptWith(secretKey))
}

func (app *ServerApp) processChannelsRequest(c *gosocketio.Channel,
	requestData models.ChannelsRequest) {
	secretKey, err := app.getClientSecretKey(c.Id(), requestData.User)
	if common.IsError(err) {
		return
	}
	pack := models.ChannelsPack{app.DB.GetChannels(requestData.User.Id)}
	c.Emit("/get-channels", pack.EncryptWith(secretKey))
}

func (app *ServerApp) PrintSessions() {
	jsonSessions, err := json.MarshalIndent(app.Sessions, "", "    ")
	if common.IsError(err) {
		log.Println(err)
	}
	fmt.Printf("Connected Users %d:\n", len(app.Sessions))
	fmt.Println(string(jsonSessions))
}

func (app *ServerApp) trySaveNewChannel(msg models.Message) bool {
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

func (app *ServerApp) createSession(socketId string, user models.User) models.Session {
	newSession := models.Session{
		User:      user,
		SecretKey: uuid.NewV4()}

	app.Sessions[socketId] = newSession
	return newSession
}

func (app *ServerApp) getClientSecretKey(socketId string, user models.User) (uuid.UUID, error) {
	session := app.Sessions[socketId]
	if session.User.Id == user.Id && session.User.Username == user.Username {
		return session.SecretKey, nil
	}
	return uuid.UUID{}, errors.New("Incorrect userId or username")
}

func (app *ServerApp) removeSession(socketId string) {
	delete(app.Sessions, socketId)
}

func (app *ServerApp) EmitToUser(userId int64, method string, data models.Encryptable) {
	// emit if user is online. To all connected clients with this account.
	// data will be encrypted.
	for socketId, session := range app.Sessions {
		if session.User.Id == userId {
			channel, err := app.Server.GetChannel(socketId)
			if !common.IsError(err) {
				channel.Emit(method, data.EncryptWith(session.SecretKey))
			} else {
				log.Println(err)
			}
		}
	}
}

func (app *ServerApp) EmitToAll(method string, data models.Encryptable) {
	// emit to all online clients.
	// data will be encrypted.
	for socketId, session := range app.Sessions {
		channel, err := app.Server.GetChannel(socketId)
		if !common.IsError(err) {
			channel.Emit(method, data.EncryptWith(session.SecretKey))
		} else {
			log.Println(err)
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
