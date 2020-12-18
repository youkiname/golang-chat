// client.go
package main

import (
	"fmt"

	"log"
	"time"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/satori/go.uuid"

	"chat/encrypt"
	"chat/gui"
	"chat/models"
	"chat/utils"
)

const WIDTH int = 1280
const HEIGHT int = 720

const GROUP_CHANNEL_TITLE = "MAIN"
const NOTES_CHANNEL_TITLE = "NOTES"

type ChatApplication struct {
	Client        *gosocketio.Client
	Connected     bool
	CurrentUser   models.User
	CommonKey     uuid.UUID
	SecretKey     uuid.UUID // key for personal channels
	LoggedIn      bool
	CurrentChatId int64
	Channels      []models.Channel
	Gui           *gui.ChatGui
}

func (chatApp *ChatApplication) init() {
	// Creates main window
	chatApp.Gui = gui.NewChatGui()
	chatApp.CommonKey = uuid.FromBytesOrNil([]byte(utils.COMMON_SECRET_KEY))

	chatApp.CurrentChatId = 0 // main channel
	chatApp.Connected = false
	chatApp.LoggedIn = false
}

func (chatApp *ChatApplication) startReconnectionTrying() {
	// if connection was lost this function
	// will try reconnect after 10s, 1m, 2m, 3m, etc...
	time.Sleep(10 * time.Second)
	for i := 0; i < 5; i++ {
		if i != 0 {
			sleepDuration, _ := time.ParseDuration(fmt.Sprintf("%dm", i))
			time.Sleep(sleepDuration)
		}
		host, port := utils.GetHostDataFromSettingsFile()
		fmt.Printf("Try reconnect to: %s:%d\n", host, port)
		if chatApp.connect(host, port, true) {
			chatApp.Gui.ShowInfo("Connection restored!")
			return
		} else {
			fmt.Printf("Unsuccess reconnect to: %s:%d\nNext try after %d minutes\n",
				host, port, i+1)
		}
	}
	chatApp.Gui.ShowInfo("We couldn't restore connection :(\n" +
		"Please, try change host information in settings file " +
		"and restart application.")
}

func (chatApp *ChatApplication) connect(host string, port int, isReconnect bool) bool {
	client, err := gosocketio.Dial(
		gosocketio.GetUrl(host, port, false),
		transport.GetDefaultWebsocketTransport())

	if utils.IsError(err) {
		if !isReconnect {
			info := fmt.Sprintf("Can't connect to host \"%s:%d\"\n"+
				"Next try after: 10 sec\n"+
				"Description: %s\n", host, port, err.Error())

			chatApp.Gui.ShowError(info)
			go chatApp.startReconnectionTrying()
		}
		return false
	}

	chatApp.Client = client

	chatApp.Connected = true
	chatApp.Gui.SetOnClose(chatApp.Client.Close)

	chatApp.initGuiCallbacks()
	chatApp.initClientCallbacks()
	return true
}

func (chatApp *ChatApplication) initGuiCallbacks() {
	chatApp.Gui.SetCallbacks(
		chatApp.sendMessage,
		func() {
			chatApp.openChannel(utils.GROUP_CHAT_ID)
		},
		func() {
			chatApp.openChannel(chatApp.CurrentUser.Id)
		},
		func(title string) {
			channelId := chatApp.getChannelId(title)
			chatApp.openChannel(channelId)
		},
		func(user models.User) {
			if user.Id == chatApp.CurrentUser.Id { // NOTES CHANNEL
				chatApp.Gui.SelectChannel(gui.NOTES_CHANNEL_TITLE)
				return
			}

			if !chatApp.isChannelInList(user.Id) {
				newChannel := models.Channel{user.Id, user.Username}
				chatApp.Gui.AppendChannel(newChannel.Title)
				chatApp.Channels = append(chatApp.Channels, newChannel)
			}
			chatApp.Gui.SelectChannel(user.Username)
		},
		chatApp.sendLoginData,
		chatApp.sendRegisterData)
}

func (chatApp *ChatApplication) initClientCallbacks() {
	// sets callbacks to socket.io client
	client := chatApp.Client

	client.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		chatApp.Gui.DisableLoginButtons()
		chatApp.Gui.DisableSend()
		chatApp.Gui.ShowError("Disconnected!")
	})

	client.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		log.Println("Connected")
		chatApp.Gui.EnableLoginButtons()
	})

	client.On("/failed-login", chatApp.processFailedAuth)
	client.On("/failed-registeration", chatApp.processFailedAuth)

	client.On("/login", chatApp.processSuccessfulLogin)

	client.On("/message", chatApp.processNewMessage)

	client.On("/get-messages", chatApp.processMessagesReceiving)
	client.On("/get-channels", chatApp.processChannelsReceiving)
}

func (chatApp *ChatApplication) processSuccessfulLogin(h *gosocketio.Channel,
	encryptedAuthData string) {
	log.Println("LOGIN")
	authData := models.SuccessfulAuth{}
	encrypt.Decrypt(chatApp.CommonKey, encryptedAuthData, &authData)
	chatApp.CurrentUser = authData.User
	chatApp.SecretKey = authData.SecretKey
	chatApp.LoggedIn = true
	chatApp.CurrentChatId = utils.GROUP_CHAT_ID

	chatApp.Gui.SetProfileInfo(authData.User.Username)

	chatApp.Gui.ClearMessages()
	chatApp.loadChannels()
	chatApp.loadMessages(chatApp.CurrentChatId)
	chatApp.Gui.EnableSend()
}

func (chatApp *ChatApplication) processFailedAuth(h *gosocketio.Channel,
	errorData models.AuthError) {
	log.Println(errorData.Description)
	if errorData.Process == "login" {
		chatApp.Gui.ShowLoginDialog(errorData.Description)
	} else {
		chatApp.Gui.ShowRegisterDialog(errorData.Description)
	}
	chatApp.LoggedIn = false
}

func (chatApp *ChatApplication) processNewMessage(h *gosocketio.Channel,
	encryptedMessage string) {
	// adds new message to list after obtaing data from server
	msg := models.SavedMessage{}
	encrypt.Decrypt(chatApp.SecretKey, encryptedMessage, &msg)

	if chatApp.canDisplayNewMessage(msg) {
		chatApp.Gui.AddMessage(msg)
	}
}

func (chatApp *ChatApplication) processMessagesReceiving(h *gosocketio.Channel,
	encryptedPack string) {
	messagesPack := models.SavedMessagesPack{}
	encrypt.Decrypt(chatApp.SecretKey, encryptedPack, &messagesPack)
	messages := messagesPack.Messages
	fmt.Printf("Got Messages. count = %d\n", len(messages))
	chatApp.Gui.SetMessages(messages)
}

func (chatApp *ChatApplication) processChannelsReceiving(h *gosocketio.Channel,
	encryptedPack string) {
	channelsPack := models.ChannelsPack{}
	encrypt.Decrypt(chatApp.SecretKey, encryptedPack, &channelsPack)
	channels := channelsPack.Channels
	fmt.Printf("Got channels. count = %d\n", len(channels))
	chatApp.Channels = channels
	chatApp.Gui.SetChannels(channels)
}

func (chatApp *ChatApplication) sendLoginData(username string, password string) {
	// sends new login data to server
	authData := models.AuthRequest{username, encrypt.GetPasswordHash(password)}
	chatApp.Client.Emit("/login", encrypt.Encrypt(chatApp.CommonKey, authData))
}

func (chatApp *ChatApplication) sendRegisterData(username string, password string) {
	// sends new registration data to server
	authData := models.AuthRequest{username, encrypt.GetPasswordHash(password)}
	chatApp.Client.Emit("/register", encrypt.Encrypt(chatApp.CommonKey, authData))
}

func (chatApp *ChatApplication) sendMessage(text string) {
	// sends new message data to server
	user := chatApp.CurrentUser
	if chatApp.Connected && chatApp.LoggedIn {
		msg := models.Message{user, chatApp.CurrentChatId, text}
		chatApp.Client.Emit("/message", encrypt.Encrypt(chatApp.SecretKey, msg))
	} else if !chatApp.LoggedIn {
		chatApp.Gui.ShowError("You are not logged in.")
	} else {
		chatApp.Gui.ShowError("You are not connected to the server.")
	}
}

func (chatApp *ChatApplication) loadMessages(chatId int64) {
	// sends getting messages request to server
	if !chatApp.Connected || !chatApp.LoggedIn {
		return
	}
	fmt.Printf("Load messages from chatId = %d\n", chatId)
	chatApp.Client.Emit("/get-messages",
		models.MessagesRequest{chatId, chatApp.CurrentUser})
}

func (chatApp *ChatApplication) loadChannels() {
	// sends gettings channels list request to server
	log.Println("Load channels")
	chatApp.Client.Emit("/get-channels", models.ChannelsRequest{chatApp.CurrentUser})
}

func (chatApp *ChatApplication) canDisplayNewMessage(msg models.SavedMessage) bool {
	// returns true if obtained message suit for displayed channel
	currentChatId := chatApp.CurrentChatId
	currentUserId := chatApp.CurrentUser.Id
	chatType := msg.GetChatType()

	isMessageFromMe := msg.User.Id == chatApp.CurrentUser.Id
	isMessageToMe := msg.ChatId == chatApp.CurrentUser.Id
	// notes message: recipient and sender is same person
	isNotesMessage := isMessageFromMe && isMessageToMe

	if chatType == "private" {
		return isNotesMessage && currentChatId == currentUserId ||
			isMessageFromMe && currentChatId == msg.ChatId ||
			isMessageToMe && currentChatId == msg.User.Id
	} else { // message to group chat
		return chatApp.CurrentChatId == utils.GROUP_CHAT_ID
	}
}

func (chatApp *ChatApplication) isChannelInList(channelId int64) bool {
	// returns true if obtained channelId is in channels list
	list := chatApp.Channels
	for _, c := range list {
		if c.Id == channelId {
			return true
		}
	}
	return false
}

func (chatApp *ChatApplication) getChannelId(title string) int64 {
	// returns channelId by title saved in channels list
	for _, channel := range chatApp.Channels {
		if channel.Title == title {
			return channel.Id
		}
	}
	return -1
}

func (chatApp *ChatApplication) openChannel(chatId int64) {
	chatApp.CurrentChatId = chatId
	chatApp.loadMessages(chatId)
}

func (chatApp *ChatApplication) openNotesChannel() {
	chatApp.openChannel(chatApp.CurrentUser.Id)
}

func main() {
	chatApp := ChatApplication{}
	chatApp.init()
	host, port := utils.GetHostDataFromSettingsFile()
	go chatApp.connect(host, port, false)
	chatApp.Gui.ShowWindow()
}
