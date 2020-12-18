// chat_gui.go
package gui

import (
	"errors"
	"fmt"

	"log"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/container"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"

	"chat/models"
)

const WIDTH int = 1280
const HEIGHT int = 720

const GROUP_CHANNEL_TITLE = "MAIN"
const NOTES_CHANNEL_TITLE = "NOTES"

type ChatGui struct {
	App                 fyne.App
	Window              fyne.Window
	LeftSideBar         *widget.Group
	MessagesList        *MessageList
	MessageListScroller *widget.ScrollContainer

	SendButton         *widget.Button
	LoginButton        *widget.Button
	RegisterButton     *widget.Button
	ProfileInfo        *widget.Label
	ChannelsRadioGroup *widget.RadioGroup

	OnGroupChannelSelect func()
	OnNotesChannelSelect func()
	OnChannelSelect      func(channelTitle string)
	OnUsernameSelect     func(models.User)

	OnSendClick func(messageText string)

	OnLoginSubmit        func(username string, password string)
	OnRegistratoinSubmit func(username string, password string)
}

func NewChatGui() *ChatGui {
	gui := &ChatGui{}

	gui.App = app.New()
	window := gui.App.NewWindow("Golang chat")
	window.Resize(fyne.NewSize(WIDTH, HEIGHT))

	window.SetContent(buildMainWindow(gui))
	window.SetMaster()

	gui.Window = window
	return gui
}

func (gui *ChatGui) SetCallbacks(
	OnSendClick func(string),
	OnGroupChannelSelect func(),
	OnNotesChannelSelect func(),
	OnChannelSelect func(string),
	OnUsernameSelect func(models.User),
	OnLoginSubmit func(string, string),
	OnRegistratoinSubmit func(string, string)) {
	gui.OnSendClick = OnSendClick
	gui.OnGroupChannelSelect = OnGroupChannelSelect
	gui.OnNotesChannelSelect = OnNotesChannelSelect
	gui.OnChannelSelect = OnChannelSelect
	gui.OnUsernameSelect = OnUsernameSelect
	gui.OnLoginSubmit = OnLoginSubmit
	gui.OnRegistratoinSubmit = OnRegistratoinSubmit
}

func (gui *ChatGui) SetOnClose(onClose func()) {
	gui.Window.SetOnClosed(onClose)
}

func (gui *ChatGui) ShowWindow() {
	// shows main window
	gui.Window.ShowAndRun()
}

func (gui *ChatGui) ShowInfo(text string) {
	dialog.ShowInformation("INFO", text, gui.Window)
}

func (gui *ChatGui) ShowError(text string) {
	// shows child window with error info
	log.Println(text)
	dialog.ShowError(errors.New(text), gui.Window)
}

func (gui *ChatGui) AddMessage(msg models.SavedMessage) {
	gui.MessagesList.AddMessage(msg)
}

func (gui *ChatGui) SetMessages(messages []models.SavedMessage) {
	list := gui.MessagesList
	list.Clear()
	list.SetMessages(messages)
	list.Refresh()
	gui.MessageListScroller.ScrollToBottom()
}

func (gui *ChatGui) ClearMessages() {
	gui.MessagesList.Clear()
}

func (gui *ChatGui) AppendChannel(title string) {
	gui.ChannelsRadioGroup.Append(title)
}

func (gui *ChatGui) SetChannels(channels []models.Channel) {
	var stringChannels []string
	stringChannels = append(stringChannels, GROUP_CHANNEL_TITLE, NOTES_CHANNEL_TITLE)
	for _, channel := range channels {
		stringChannels = append(stringChannels, channel.Title)
	}
	gui.ChannelsRadioGroup.Options = stringChannels
	gui.ChannelsRadioGroup.Refresh()
}

func (gui *ChatGui) SelectChannel(title string) {
	gui.ChannelsRadioGroup.SetSelected(title)
}

func (gui *ChatGui) EnableLoginButtons() {
	gui.LoginButton.Enable()
	gui.RegisterButton.Enable()
}

func (gui *ChatGui) DisableLoginButtons() {
	gui.LoginButton.Disable()
	gui.RegisterButton.Disable()
}

func (gui *ChatGui) EnableSend() {
	gui.SendButton.Enable()
}

func (gui *ChatGui) DisableSend() {
	gui.SendButton.Disable()
}

func (gui *ChatGui) SetProfileInfo(username string) {
	gui.ProfileInfo.SetText("WELCOME, " + username)
}

func (gui *ChatGui) processSend(inputText string) {
	if inputText != "" && !gui.SendButton.Disabled() {
		gui.OnSendClick(inputText)
		gui.MessageListScroller.ScrollToBottom()
	}
}

// -------- CHILD WINDOWS ----------

func (gui *ChatGui) ShowLoginDialog(title string) {
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
				gui.OnLoginSubmit(inputUsername.Text, inputPassword.Text)
			}
		}, gui.Window)
}

func (gui *ChatGui) ShowRegisterDialog(title string) {
	// creates and shows child window with registration form
	inputUsername := widget.NewEntry()
	inputUsername.SetPlaceHolder("username")
	inputPassword := widget.NewPasswordEntry()
	inputPassword.SetPlaceHolder("password")

	loginBox := widget.NewVBox(inputUsername, inputPassword)
	loginBox.Resize(fyne.NewSize(400, 400))
	title = title + "\n- username must be less than 20.\n- username mustn't has spaces."
	dialog.ShowCustomConfirm(title, "Ok", "Cancel", loginBox,
		func(result bool) {
			if result {
				gui.OnRegistratoinSubmit(inputUsername.Text, inputPassword.Text)
			}
		}, gui.Window)
}

// -------- BUILD ----------

func buildLeftSidebar(gui *ChatGui) *widget.Group {
	// creates login and register page
	// sets login and register buttons callbacks
	gui.LoginButton = widget.NewButton("Login", func() {
		gui.ShowLoginDialog("Login")
	})
	gui.RegisterButton = widget.NewButton("Register", func() {
		gui.ShowRegisterDialog("Registration")
	})

	gui.ProfileInfo = widget.NewLabel("")

	group := widget.NewGroup("Profile",
		gui.LoginButton, gui.RegisterButton, gui.ProfileInfo)
	group.Resize(fyne.NewSize(400, HEIGHT))
	return group
}

func buildCenter(gui *ChatGui) *widget.Group {
	// creates messenger page: messages list and text input
	messagesList := NewMessageList(func(user models.User) {
		gui.OnUsernameSelect(user)
	})
	gui.MessagesList = messagesList
	scroller := widget.NewScrollContainer(messagesList.GetContainer())
	scroller.SetMinSize(fyne.NewSize(500, 600))
	gui.MessageListScroller = scroller

	input := NewEnterEntry()
	input.SetOnEnter(func() {
		gui.processSend(input.Text)
		input.Clear()
	})
	input.SetPlaceHolder("Your message")

	gui.SendButton = widget.NewButton("Send", func() {
		gui.processSend(input.Text)
		input.Clear()
	})
	inputForm := widget.NewHBox(input, gui.SendButton)

	mainContainer := container.NewVBox(container.NewMax(scroller), widget.NewSeparator(), inputForm)

	return widget.NewGroup("Messenger", mainContainer)
}

func buildRightSidebar(gui *ChatGui) *widget.Group {
	// creates radio group with channel selecting callback
	var stringChannels []string
	radioGroup := widget.NewRadioGroup(stringChannels, func(changed string) {
		fmt.Printf("Select channel = %s\n", changed)
		if changed == GROUP_CHANNEL_TITLE {
			gui.OnGroupChannelSelect()
		} else if changed == NOTES_CHANNEL_TITLE {
			gui.OnNotesChannelSelect()
		} else {
			gui.OnChannelSelect(changed)
		}
	})
	radioGroup.Required = true
	gui.ChannelsRadioGroup = radioGroup
	return widget.NewGroup("Channels", radioGroup)
}

func buildMainWindow(gui *ChatGui) *fyne.Container {
	// returns container with messenger page and sidebars
	leftSideBar := buildLeftSidebar(gui)
	gui.LeftSideBar = leftSideBar

	center := buildCenter(gui)

	rightSideBar := buildRightSidebar(gui)
	gui.DisableLoginButtons() // disable by default. Waiting successful connect
	gui.DisableSend()
	return fyne.NewContainerWithLayout(
		layout.NewBorderLayout(nil, nil, leftSideBar, rightSideBar),
		leftSideBar,
		center, rightSideBar)
}
