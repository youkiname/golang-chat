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

const WIDTH int = 640
const HEIGHT int = 480

var currentUser User
var loggedIn = false
var client *gosocketio.Client

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

func sendMessage(client *gosocketio.Client, user User, text string) {
	client.Emit("/message", Message{user, text})
}

func addMessageToList(list *widget.Box, msg Message) {
	messageLabel := widget.NewLabel(msg.User.Username + ": " + msg.Text)
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

func showLoginDialog(window fyne.Window) {
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))

	dialog.ShowCustomConfirm("Login", "Ok", "cancel", loginBox,
		func(result bool) {
			if result {
				sendLoginData(client, inputUsername.Text, inputPassword.Text)
			}
		}, window)
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

	client.On("/message", func(h *gosocketio.Channel, msg Message) {
		addMessageToList(messagesList, msg)
	})

	return widget.NewVBox(messagesList, input, send, systemMessagesList)
}

func buildRightSidebar() fyne.Widget {
	channels := []fyne.Widget{
		widget.NewLabel("Channel 1"),
		widget.NewLabel("Channel 2"),
	}
	box := widget.NewVBox()
	box.Resize(fyne.NewSize(100, 470))

	for i := 0; i < len(channels); i++ {
		box.Append(channels[i])
	}
	return box
}

func buildMainWindow(app fyne.App, window fyne.Window) fyne.Widget {
	return widget.NewHBox(buildCenter(), buildRightSidebar())
}

// ------------------

func main() {
	app := app.New()

	client = connect()

	window := app.NewWindow("Super chat")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))

	window.SetContent(buildMainWindow(app, window))
	window.SetMaster()
	window.SetOnClosed(func() {
		client.Close()
	})

	// showLoginDialog(window)
	client.Emit("/login", LoginData{"vadim", "202cb962ac59075b964b07152d234b70"})

	window.ShowAndRun()
}
