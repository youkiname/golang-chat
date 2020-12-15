// client.go
package main

import (
	"errors"
	"fmt"

	"log"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/satori/go.uuid"

	"chat/common"
	"chat/gui"
	"chat/models"
)

const WIDTH int = 1280
const HEIGHT int = 720

const GROUP_CHANNEL_TITLE = "MAIN"
const NOTES_CHANNEL_TITLE = "NOTES"

type ChatApplication struct {
	App                 fyne.App
	Window              fyne.Window
	LeftSideBar         *widget.Group
	MessagesList        *gui.MessageList
	MessageListScroller *widget.ScrollContainer
	Client              *gosocketio.Client
	Connected           bool
	CurrentUser         models.User
	CommonKey           uuid.UUID // common key for group channel
	SecretKey           uuid.UUID // key for personal channels
	LoggedIn            bool
	CurrentChatId       int64
	ProfileInfo         *widget.Label
	Channels            []models.Channel
	ChannelsRadioGroup  *widget.RadioGroup
}

func (chatApp *ChatApplication) init() {
	// Creates main window
	chatApp.App = app.New()
	window := chatApp.App.NewWindow("Golang chat")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))

	window.SetContent(buildMainWindow(chatApp))
	window.SetMaster()

	chatApp.Window = window
	chatApp.CurrentChatId = 0 // main channel
	chatApp.Connected = false
	chatApp.LoggedIn = false
}

func (chatApp *ChatApplication) showWindow() {
	// shows main window
	chatApp.Window.ShowAndRun()
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
		host, port := common.GetHostDataFromSettingsFile()
		fmt.Printf("Try reconnect to: %s:%d\n", host, port)
		if chatApp.connect(host, port, true) {
			dialog.ShowInformation("Success!", "Connection restored!", chatApp.Window)
			return
		} else {
			fmt.Printf("Unsuccess reconnect to: %s:%d\nNext try after %d minutes\n",
				host, port, i+1)
		}
	}
	dialog.ShowInformation(":(", "We couldn't restore connection :(\n"+
		"Please, try change host information in settings file "+
		"and restart application.", chatApp.Window)
}

func (chatApp *ChatApplication) connect(host string, port int, isReconnect bool) bool {
	client, err := gosocketio.Dial(
		gosocketio.GetUrl(host, port, false),
		transport.GetDefaultWebsocketTransport())

	if common.IsError(err) {
		if !isReconnect {
			info := fmt.Sprintf("Can't connect to host \"%s:%d\"\n"+
				"Next try after: 10 sec\n"+
				"Description: %s\n", host, port, err.Error())

			chatApp.showError(info)
			go chatApp.startReconnectionTrying()
		}
		return false
	}

	err = client.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		chatApp.showError("Disconnected!")
	})
	if common.IsError(err) {
		chatApp.showError(err.Error())
		return false
	}

	err = client.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		log.Println("Connected")
	})
	if common.IsError(err) {
		chatApp.showError(err.Error())
		return false
	}

	chatApp.Client = client

	chatApp.Connected = true
	chatApp.Window.SetOnClosed(func() {
		chatApp.Client.Close()
	})

	chatApp.initClientCallbacks()
	return true
}

func (chatApp *ChatApplication) initClientCallbacks() {
	// sets callbacks to socket.io client
	client := chatApp.Client
	client.On("/failed-login", chatApp.processFailedAuth)
	client.On("/failed-registeration", chatApp.processFailedAuth)

	client.On("/login", chatApp.processSuccessfulLogin)

	client.On("/message", chatApp.processNewMessage)

	client.On("/get-messages", chatApp.processMessagesReceiving)
	client.On("/get-channels", chatApp.processChannelsReceiving)
}

func (chatApp *ChatApplication) processSuccessfulLogin(h *gosocketio.Channel,
	authData models.SuccessfulAuth) {
	log.Println("LOGIN")
	chatApp.CurrentUser = authData.User
	chatApp.SecretKey = authData.SecretKey
	chatApp.LoggedIn = true
	chatApp.CurrentChatId = common.GROUP_CHAT_ID

	chatApp.ProfileInfo.SetText("WELCOME, " + authData.User.Username)
	chatApp.LeftSideBar.Append(widget.NewLabel("Success Login: " + authData.User.Username))

	chatApp.clearMessagesList()
	chatApp.loadChannels()
	chatApp.loadMessages(chatApp.CurrentChatId)
}

func (chatApp *ChatApplication) processFailedAuth(h *gosocketio.Channel,
	errorData models.AuthError) {
	log.Println(errorData.Description)
	if errorData.Process == "login" {
		chatApp.showLoginDialog(errorData.Description)
	} else {
		chatApp.showRegisterDialog(errorData.Description)
	}
	chatApp.LoggedIn = false
	chatApp.ProfileInfo.SetText("FAILED LOGIN")
}

func (chatApp *ChatApplication) processNewMessage(h *gosocketio.Channel,
	encryptedMessage string) {
	// adds new message to list after obtaing data from server
	msg := models.DecryptSavedMessage(chatApp.SecretKey, encryptedMessage)

	if chatApp.canDisplayNewMessage(msg) {
		chatApp.addMessageToList(msg)
	}
}

func (chatApp *ChatApplication) processMessagesReceiving(h *gosocketio.Channel,
	encryptedPack string) {
	messagesPack := models.DecryptSavedMessagePack(chatApp.SecretKey, encryptedPack)
	messages := messagesPack.Messages
	fmt.Printf("Got Messages. count = %d\n", len(messages))
	chatApp.clearMessagesList()
	chatApp.MessagesList.SetMessages(messages)
	chatApp.MessagesList.Refresh()
	chatApp.MessageListScroller.ScrollToBottom()
}

func (chatApp *ChatApplication) processChannelsReceiving(h *gosocketio.Channel,
	encryptedPack string) {
	channelsPack := models.DecryptChannelsPack(chatApp.SecretKey, encryptedPack)
	channels := channelsPack.Channels
	fmt.Printf("Got channels. count = %d\n", len(channels))
	chatApp.Channels = channels
	chatApp.refreshChannelList()
	if chatApp.CurrentChatId == common.GROUP_CHAT_ID {
		chatApp.ChannelsRadioGroup.SetSelected(GROUP_CHANNEL_TITLE)
	}
}

func (chatApp *ChatApplication) sendLoginData(username string, password string) {
	// sends new login data to server
	if chatApp.Connected {
		chatApp.Client.Emit("/login",
			models.AuthRequest{username, common.GetPasswordHash(password)})
	} else {
		chatApp.showError("You are not connected to the server.")
	}
}

func (chatApp *ChatApplication) sendRegisterData(username string, password string) {
	// sends new registration data to server
	if chatApp.Connected {
		chatApp.Client.Emit("/register",
			models.AuthRequest{username, common.GetPasswordHash(password)})
	} else {
		chatApp.showError("You are not connected to the server.")
	}
}

func (chatApp *ChatApplication) sendMessage(text string) {
	// sends new message data to server
	user := chatApp.CurrentUser
	if chatApp.Connected && chatApp.LoggedIn {
		chatApp.Client.Emit("/message", models.Message{user, chatApp.CurrentChatId, text})
	} else if !chatApp.LoggedIn {
		chatApp.showError("You are not logged in.")
	} else {
		chatApp.showError("You are not connected to the server.")
	}
}

func (chatApp *ChatApplication) loadMessages(chatId int64) {
	// sends getting messages request to server
	if !chatApp.Connected || !chatApp.LoggedIn {
		return
	}
	fmt.Printf("Load messages from chatId = %d\n", chatId)
	client := chatApp.Client
	client.Emit("/get-messages", models.MessagesRequest{chatId, chatApp.CurrentUser})
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
		return chatApp.CurrentChatId == common.GROUP_CHAT_ID
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
	if title == NOTES_CHANNEL_TITLE {
		return chatApp.CurrentUser.Id
	}
	return common.GROUP_CHAT_ID // MAIN CHANNEL
}

func (chatApp *ChatApplication) openChannel(chatId int64) {
	chatApp.CurrentChatId = chatId
	chatApp.loadMessages(chatId)
}

func (chatApp *ChatApplication) openNotesChannel() {
	chatApp.openChannel(chatApp.CurrentUser.Id)
}

func (chatApp *ChatApplication) addMessageToList(msg models.SavedMessage) {
	chatApp.MessagesList.AddMessage(msg)
	chatApp.MessagesList.Refresh()

}

func (chatApp *ChatApplication) clearMessagesList() {
	chatApp.MessagesList.Clear()
	chatApp.MessagesList.Refresh()
}

func (chatApp *ChatApplication) showError(description string) {
	// shows child window with error info
	log.Println(description)
	dialog.ShowError(errors.New(description), chatApp.Window)
}

func (chatApp *ChatApplication) refreshChannelList() {
	// refresh left sidebar with channels list
	// --------------------------------
	// Use this function only after replacing all channels.
	// For example, after new person login.
	// After appending (group.Append) this func will be called automatically.
	var stringChannels []string
	stringChannels = append(stringChannels, GROUP_CHANNEL_TITLE, NOTES_CHANNEL_TITLE)
	for _, channel := range chatApp.Channels {
		stringChannels = append(stringChannels, channel.Title)
	}
	chatApp.ChannelsRadioGroup.Options = stringChannels
	chatApp.ChannelsRadioGroup.Refresh()
}

func (chatApp *ChatApplication) openChannelByUser(user models.User) {
	// selects obtained username in channels radio group
	// or creates new channel if this username was not be added before
	channelsGroup := chatApp.ChannelsRadioGroup

	if user.Id == chatApp.CurrentUser.Id { // NOTES CHANNEL
		channelsGroup.SetSelected(NOTES_CHANNEL_TITLE)
		return
	}

	if !chatApp.isChannelInList(user.Id) {
		channelsGroup.Append(user.Username)
		chatApp.Channels = append(chatApp.Channels, models.Channel{user.Id, user.Username})
	}
	channelsGroup.SetSelected(user.Username)
}

// -------- BUILD WINDOW----------

func (chatApp *ChatApplication) showLoginDialog(title string) {
	// creates and shows child window with login form
	inputUsername := widget.NewEntry()
	inputUsername.SetPlaceHolder("username")
	inputPassword := widget.NewPasswordEntry()
	inputPassword.SetPlaceHolder("password")

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))

	dialog.ShowCustomConfirm(title, "Ok", "Cancel", loginBox,
		func(result bool) {
			if result {
				chatApp.sendLoginData(inputUsername.Text, inputPassword.Text)
			}
		}, chatApp.Window)
}

func (chatApp *ChatApplication) showRegisterDialog(title string) {
	// creates and shows child window with registration form
	inputUsername := widget.NewEntry()
	inputUsername.SetPlaceHolder("username")
	inputPassword := widget.NewPasswordEntry()
	inputPassword.SetPlaceHolder("password")

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))

	dialog.ShowCustomConfirm(title, "Ok", "Cancel", loginBox,
		func(result bool) {
			if result {
				chatApp.sendRegisterData(inputUsername.Text, inputPassword.Text)
			}
		}, chatApp.Window)
}

func buildLeftSidebar(chatApp *ChatApplication) *widget.Group {
	// creates login and register page
	// sets login and register buttons callbacks
	login := widget.NewButton("Login", func() {
		if chatApp.Connected {
			chatApp.showLoginDialog("Login")
		} else {
			chatApp.showError("You are not connected to the server.")
		}
	})
	register := widget.NewButton("Register", func() {
		if chatApp.Connected {
			chatApp.showRegisterDialog("Registration")
		} else {
			chatApp.showError("You are not connected to the server.")
		}
	})

	// info about previous users logged in
	info := widget.NewLabel("")
	chatApp.ProfileInfo = info

	group := widget.NewGroup("Profile", login, register, info)
	group.Resize(fyne.NewSize(400, HEIGHT))
	return group
}

func buildCenter(chatApp *ChatApplication) *fyne.Container {
	// creates messenger page: messages list and text input
	messagesList := gui.NewMessageList(func(user models.User) {
		chatApp.openChannelByUser(user)
	})
	chatApp.MessagesList = messagesList
	scroller := widget.NewScrollContainer(messagesList.GetContainer())
	scroller.SetMinSize(fyne.NewSize(500, 500))
	chatApp.MessageListScroller = scroller

	input := widget.NewEntry()
	input.SetPlaceHolder("Your message")

	send := widget.NewButton("Send", func() {
		if input.Text != "" {
			chatApp.sendMessage(input.Text)
			input.SetText("")
		}
	})

	top := widget.NewGroup("Messenger", scroller)
	bottom := widget.NewHBox(input, send)

	layout := layout.NewBorderLayout(top, bottom, nil, nil)

	return fyne.NewContainerWithLayout(layout, top, bottom)
}

func buildRightSidebar(chatApp *ChatApplication) fyne.Widget {
	// creates radio group with channel selecting callback
	var stringChannels []string
	radioGroup := widget.NewRadioGroup(stringChannels, func(changed string) {
		fmt.Printf("Select channel = %s\n", changed)
		selectedChatId := chatApp.getChannelId(changed)
		chatApp.openChannel(selectedChatId)
	})
	radioGroup.Required = true
	chatApp.ChannelsRadioGroup = radioGroup
	return widget.NewGroup("Channels", radioGroup)
}

func buildMainWindow(chatApp *ChatApplication) *fyne.Container {
	// returns container with messenger page and sidebars
	leftSideBar := buildLeftSidebar(chatApp)
	chatApp.LeftSideBar = leftSideBar

	center := buildCenter(chatApp)

	rightSideBar := buildRightSidebar(chatApp)
	return fyne.NewContainerWithLayout(
		layout.NewBorderLayout(nil, nil, leftSideBar, rightSideBar),
		leftSideBar,
		center, rightSideBar)
}

func main() {
	chatApp := ChatApplication{}
	chatApp.init()
	host, port := common.GetHostDataFromSettingsFile()
	go chatApp.connect(host, port, false)
	chatApp.showWindow()
}
