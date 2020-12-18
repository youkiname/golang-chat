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

	"chat/db"
	"chat/encrypt"
	"chat/models"
	"chat/utils"
)

const FAILED_LOGIN_LIMIT int = 5
const FAILED_LOGIN_TIME_LIMIT int = 2 * 60 // 2 minutes

type ServerApp struct {
	Server    *gosocketio.Server
	Sessions  map[string]models.Session // map: socket ID -> Session data
	CommonKey uuid.UUID
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
	if lastFailedLoginDate+int64(FAILED_LOGIN_TIME_LIMIT) < utils.GetTimestampNow() {
		db.ClearFailedLogin(userId)
		return FAILED_LOGIN_LIMIT
	}
	return FAILED_LOGIN_LIMIT - countFailedLogin
}

func (app *ServerApp) Init() {
	// Connects to db. Generate utils key (for group channel)
	// And sets callbacks to socket.io server
	app.Sessions = make(map[string]models.Session)
	app.CommonKey = uuid.FromBytesOrNil([]byte(utils.COMMON_SECRET_KEY))
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

func (app *ServerApp) processNewLogin(c *gosocketio.Channel, encryptedAuthData string) {
	authData := models.AuthRequest{}
	encrypt.Decrypt(app.CommonKey, encryptedAuthData, &authData)
	user, err := app.DB.GetUserByName(authData.Username)
	isUsernameValid := !utils.IsError(err)

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
	c.Join("main")
	authData := models.SuccessfulAuth{User: user, SecretKey: newSession.SecretKey}
	c.Emit("/login", encrypt.Encrypt(app.CommonKey, authData))
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

func (app *ServerApp) processNewRegistration(c *gosocketio.Channel, encryptedAuthData string) {
	authData := models.AuthRequest{}
	encrypt.Decrypt(app.CommonKey, encryptedAuthData, &authData)
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

func (app *ServerApp) processNewMessage(c *gosocketio.Channel, encryptedMessage string) {
	secretKey, err := app.getClientSecretKey(c.Id())
	if utils.IsError(err) {
		return
	}
	msg := models.Message{}
	encrypt.Decrypt(secretKey, encryptedMessage, &msg)

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
	c.Emit("/message", encrypt.Encrypt(secretKey, savedMessage))
}

func (app *ServerApp) processMessagesRequest(c *gosocketio.Channel,
	requestData models.MessagesRequest) {
	secretKey, err := app.getClientSecretKey(c.Id())
	if utils.IsError(err) {
		return
	}
	user := requestData.User
	chatId := requestData.ChatId
	messages := app.DB.GetMessagesFromChat(user.Id, chatId)
	pack := models.SavedMessagesPack{messages}
	c.Emit("/get-messages", encrypt.Encrypt(secretKey, pack))
}

func (app *ServerApp) processChannelsRequest(c *gosocketio.Channel,
	requestData models.ChannelsRequest) {
	secretKey, err := app.getClientSecretKey(c.Id())
	if utils.IsError(err) {
		return
	}
	pack := models.ChannelsPack{app.DB.GetChannels(requestData.User.Id)}
	c.Emit("/get-channels", encrypt.Encrypt(secretKey, pack))
}

func (app *ServerApp) PrintSessions() {
	jsonSessions, err := json.MarshalIndent(app.Sessions, "", "    ")
	if utils.IsError(err) {
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

func (app *ServerApp) getClientSecretKey(socketId string) (uuid.UUID, error) {
	session, ok := app.Sessions[socketId]
	if !ok { // session does not exist
		return uuid.UUID{}, errors.New("Incorrect userId or username")
	}
	return session.SecretKey, nil
}

func (app *ServerApp) removeSession(socketId string) {
	delete(app.Sessions, socketId)
}

func (app *ServerApp) EmitToUser(userId int64, method string, data interface{}) {
	// emit if user is online. To all connected clients with this account.
	// data will be encrypted.
	for socketId, session := range app.Sessions {
		if session.User.Id == userId {
			channel, err := app.Server.GetChannel(socketId)
			if !utils.IsError(err) {
				channel.Emit(method, encrypt.Encrypt(session.SecretKey, data))
			} else {
				log.Println(err)
			}
		}
	}
}

func (app *ServerApp) EmitToAll(method string, data interface{}) {
	// emit to all online clients.
	// data will be encrypted.
	for socketId, session := range app.Sessions {
		channel, err := app.Server.GetChannel(socketId)
		if !utils.IsError(err) {
			channel.Emit(method, encrypt.Encrypt(session.SecretKey, data))
		} else {
			log.Println(err)
		}
	}
}

func isValid(username string) bool {
	// username must not have spaces and be less than 20
	if len(username) > 20 {
		return false
	}
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

	host, port := utils.GetHostDataFromSettingsFile()

	serverApp.Run(fmt.Sprintf("%s:%d", host, port))
}
