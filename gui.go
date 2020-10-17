// gui.go
package main

import (
	"log"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/widget"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

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

func main() {
	mainApp := app.New()
	client := connect()
	var currentUser User

	window := mainApp.NewWindow("Super chat")
	window.Resize(fyne.NewSize(640, 480))
	inputUsername := widget.NewEntry()
	inputPassword := widget.NewPasswordEntry()
	loginProgressBar := widget.NewProgressBarInfinite()
	loginProgressBar.Hide()
	loginForm := widget.NewForm(
		widget.NewFormItem("Username", inputUsername),
		widget.NewFormItem("Password", inputPassword))
	loginForm.OnSubmit = func() {
		loginProgressBar.Show()
		loginProgressBar.Start()
		sendLoginData(client, inputUsername.Text, inputPassword.Text)
	}
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
	quit := widget.NewButton("Quit", func() {
		client.Close()
		mainApp.Quit()
	})

	window.SetContent(widget.NewVBox(
		loginForm,
		loginProgressBar,
		messagesList,
		input,
		send,
		systemMessagesList,
		quit,
	))

	client.On("/login", func(h *gosocketio.Channel, user User) {
		currentUser = user
		loginForm.Hide()
		loginProgressBar.Stop()
		loginProgressBar.Hide()
		showSystemMessage(systemMessagesList, "~ SUCCESS LOGIN ~")
	})

	client.On("/message", func(h *gosocketio.Channel, msg Message) {
		addMessageToList(messagesList, msg)
	})

	window.ShowAndRun()
}
