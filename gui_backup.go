// gui.go
package main

import (
	"log"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/widget"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

const WIDTH int = 1280
const HEIGHT int = 720

type ChatApplication struct {
	App    fyne.App
	Window fyne.Window
	client *gosocketio.Client
	currentUser User
	currentChatId int64
}

func (chatApp *ChatApplication) init() {
	chatApp.App := app.New()
}


func connect() *gosocketio.Client {

	host := "localhost"
	port := 3811

	client, err := gosocketio.Dial(
		gosocketio.GetUrl(host, port, false),
		transport.GetDefaultWebsocketTransport())

	if err != nil {
		log.Fatal(err)
	}

	err = client.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
		log.Fatal("Disconnected")
	})
	if err != nil {
		log.Fatal(err)
	}

	err = client.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
		log.Println("Connected")
	})
	if err != nil {
		log.Fatal(err)
	}

	return client
}

func sendLoginData(client *gosocketio.Client, username string, password string) {
	client.Emit("/login", LoginData{username, GetMD5Hash(password)})
}

func sendRegisterData(client *gosocketio.Client, username string, password string) {
	client.Emit("/register", LoginData{username, GetMD5Hash(password)})
}

func sendMessage(client *gosocketio.Client, user User, text string) {
	client.Emit("/message", Message{user, chatId, text})
}

func addMessageToList(list *widget.Box, msg SavedMessage) {
	messageLabel := widget.NewLabel(msg.UserData.Username + ": " + msg.Text)
	list.Append(messageLabel)
}

func showSystemMessage(list *widget.Box, text string) {
	label := widget.NewLabelWithStyle(
		text,
		fyne.TextAlignCenter,
		fyne.TextStyle{true, false, true})
	list.Append(label)
}

func showError(list *widget.Box, errorText string) {
	showSystemMessage(list, "ERROR: "+errorText)
}

// -------- BUILD WINDOW----------

func showLoginDialog(window fyne.Window, title string) {
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))

	dialog.ShowCustomConfirm(title, "Ok", "Cancel", loginBox,
		func(result bool) {
			if result {
				sendLoginData(client, inputUsername.Text, inputPassword.Text)
			}
		}, window)
}

func showRegisterDialog(window fyne.Window, title string) {
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))

	dialog.ShowCustomConfirm(title, "Ok", "Cancel", loginBox,
		func(result bool) {
			if result {
				sendRegisterData(client, inputUsername.Text, inputPassword.Text)
			}
		}, window)
}

func buildLeftSidebar() fyne.Widget {
	channels := []fyne.Widget{
		widget.NewLabel("Channel 1"),
		widget.NewLabel("Channel 2"),
	}
	group := widget.NewGroup("Profile")

	for i := 0; i < len(channels); i++ {
		group.Append(channels[i])
	}
	return group
}

func buildCenter() fyne.Widget {
	messagesList := widget.NewVBox()
	input := widget.NewEntry()
	input.SetPlaceHolder("Your message")
	send := widget.NewButton("Send", func() {
		if input.Text != "" {
			go sendMessage(client, currentUser, input.Text)
			input.SetText("")
		}
	})
	systemMessagesList := widget.NewVBox()

	client.On("/login", func(h *gosocketio.Channel, user User) {
		log.Println("LOGIN")
		currentUser = user
		loggedIn = true
	})

	client.On("/message", func(h *gosocketio.Channel, msg SavedMessage) {
		addMessageToList(messagesList, msg)
	})

	return widget.NewGroup("Messenger", messagesList, input, send, systemMessagesList)
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

func buildMainWindow(app fyne.App, window fyne.Window) fyne.Widget {
	client.On("/failed-login", func(h *gosocketio.Channel, errorData LoginError) {
		log.Println(errorData.Description)
		showLoginDialog(window, errorData.Description)
		loggedIn = false
	})
	client.On("/failed-registeration", func(h *gosocketio.Channel, errorData RegistrationError) {
		log.Println(errorData.Description)
		showRegisterDialog(window, errorData.Description)
		loggedIn = false
	})
	return widget.NewHBox(buildLeftSidebar(), buildCenter(), buildRightSidebar())
}

// ------------------

func main() {
	app := app.New()

	//client = connect()

	window := app.NewWindow("Super chat")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))

	window.SetContent(buildMainWindow(app, window))
	window.SetMaster()
	window.SetOnClosed(func() {
		client.Close()
	})
	//showRegisterDialog(window, "Registration")
	//showLoginDialog(window, "Login")
	//client.Emit("/login", LoginData{"vadim", "202cb962ac59075b964b07152d234b70"})

	window.ShowAndRun()
}
