// gui.go
package main

import (
	"errors"
	"log"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/widget"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

const WIDTH int = 1280
const HEIGHT int = 720

const HOST string = "localhost"
const PORT int = 3811

type ChatApplication struct {
	App           fyne.App
	Window        fyne.Window
	LeftSideBar   *widget.Group
	MessagesList  *widget.Box
	Client        *gosocketio.Client
	Connected     bool
	CurrentUser   User
	LoggedIn      bool
	CurrentChatId int64
}

func (chatApp *ChatApplication) init() {
	chatApp.App = app.New()
	window := chatApp.App.NewWindow("Super chat")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))

	window.SetContent(buildMainWindow(chatApp))
	window.SetMaster()

	chatApp.Window = window
	chatApp.CurrentChatId = 0 // main channel
	chatApp.Connected = false
	chatApp.LoggedIn = false
}

func (chatApp *ChatApplication) showWindow() {
	chatApp.Window.ShowAndRun()
}

func (chatApp *ChatApplication) connect() bool {
	time.Sleep(1 * time.Second)
	client, err := gosocketio.Dial(
		gosocketio.GetUrl(HOST, PORT, false),
		transport.GetDefaultWebsocketTransport())

	if err != nil {
		chatApp.showError(err)
		return false
	}

	err = client.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		chatApp.showError(errors.New("Disconnected!"))
	})
	if err != nil {
		chatApp.showError(err)
		return false
	}

	err = client.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		log.Println("Connected")
	})
	if err != nil {
		chatApp.showError(err)
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
	client := chatApp.Client
	client.On("/failed-login", func(h *gosocketio.Channel, errorData LoginError) {
		log.Println(errorData.Description)
		chatApp.showLoginDialog(errorData.Description)
		chatApp.LoggedIn = false
	})
	client.On("/failed-registeration", func(h *gosocketio.Channel, errorData RegistrationError) {
		log.Println(errorData.Description)
		chatApp.showRegisterDialog(errorData.Description)
		chatApp.LoggedIn = false
	})

	client.On("/login", func(h *gosocketio.Channel, user User) {
		log.Println("LOGIN")
		chatApp.CurrentUser = user
		chatApp.LoggedIn = true
		chatApp.LeftSideBar.Append(widget.NewLabel("Success Login: " + user.Username))
	})

	client.On("/message", func(h *gosocketio.Channel, msg SavedMessage) {
		chatApp.addMessageToList(msg)
	})
}

func (chatApp *ChatApplication) sendLoginData(username string, password string) {
	if chatApp.Connected {
		chatApp.Client.Emit("/login", LoginData{username, GetMD5Hash(password)})
	} else {
		chatApp.showError(errors.New("You are not connected to the server."))
	}
}

func (chatApp *ChatApplication) sendRegisterData(username string, password string) {
	if chatApp.Connected {
		chatApp.Client.Emit("/register", LoginData{username, GetMD5Hash(password)})
	} else {
		chatApp.showError(errors.New("You are not connected to the server."))
	}
}

func (chatApp *ChatApplication) sendMessage(user User, text string) {
	if chatApp.Connected && chatApp.LoggedIn {
		chatApp.Client.Emit("/message", Message{user, chatApp.CurrentChatId, text})
	} else if !chatApp.LoggedIn {
		chatApp.showError(errors.New("You are not logged in."))
	} else {
		chatApp.showError(errors.New("You are not connected to the server."))
	}
}

func (chatApp *ChatApplication) addMessageToList(msg SavedMessage) {
	messageLabel := widget.NewLabel(msg.UserData.Username + ": " + msg.Text)
	chatApp.MessagesList.Append(messageLabel)
}

func (chatApp *ChatApplication) showError(err error) {
	log.Println(err)
	dialog.ShowError(err, chatApp.Window)
}

// -------- BUILD WINDOW----------

func (chatApp *ChatApplication) showLoginDialog(title string) {
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()

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
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()

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
	login := widget.NewButton("Login", func() {
		if chatApp.Connected {
			chatApp.showLoginDialog("Login")
		} else {
			chatApp.showError(errors.New("You are not connected to the server."))
		}
	})
	register := widget.NewButton("Register", func() {
		if chatApp.Connected {
			chatApp.showRegisterDialog("Register")
		} else {
			chatApp.showError(errors.New("You are not connected to the server."))
		}
	})

	group := widget.NewGroup("Profile", login, register)
	group.Resize(fyne.NewSize(400, HEIGHT))
	return group
}

func buildCenter(chatApp *ChatApplication) fyne.Widget {
	messagesList := widget.NewVBox()
	input := widget.NewEntry()
	input.SetPlaceHolder("Your message")
	send := widget.NewButton("Send", func() {
		if input.Text != "" {
			chatApp.sendMessage(chatApp.CurrentUser, input.Text)
			input.SetText("")
		}
	})
	chatApp.MessagesList = messagesList

	return widget.NewGroup("Messenger", messagesList, input, send)
}

func buildRightSidebar() fyne.Widget {
	channels := []fyne.Widget{
		widget.NewLabel("Channel 1"),
		widget.NewLabel("Channel 2"),
	}
	group := widget.NewGroup("Chats")
	group.Resize(fyne.NewSize(100, HEIGHT))

	for i := 0; i < len(channels); i++ {
		group.Append(channels[i])
	}
	return group
}

func buildMainWindow(chatApp *ChatApplication) fyne.Widget {
	leftSideBar := buildLeftSidebar(chatApp)
	chatApp.LeftSideBar = leftSideBar
	return widget.NewHBox(leftSideBar,
		buildCenter(chatApp),
		buildRightSidebar()))
}

// ------------------

func main() {
	chatApp := ChatApplication{}
	chatApp.init()
	go chatApp.connect()
	chatApp.showWindow()
	//showRegisterDialog(window, "Registration")
	//showLoginDialog(window, "Login")
	//client.Emit("/login", LoginData{"vadim", "202cb962ac59075b964b07152d234b70"})

}
