// custom_widgets
package gui

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"

	"chat/models"
)

var separatorColor = color.RGBA{33, 150, 243, 255}
var msgBodyColor = color.RGBA{125, 119, 119, 255}
var msgStrokeColor = color.RGBA{80, 80, 80, 255}

const MAX_MSG_TEXT_LINE_LENGTH int = 71

type tappableLabel struct {
	widget.Label
	TappedFunc func()
}

func NewTappableLabel(text string, tappedFunc func()) *tappableLabel {
	label := &tappableLabel{}
	label.TappedFunc = tappedFunc
	label.ExtendBaseWidget(label)
	label.SetText(text)
	label.TextStyle = fyne.TextStyle{true, false, false}

	return label
}

func (t *tappableLabel) Tapped(_ *fyne.PointEvent) {
	t.TappedFunc()
}

func (t *tappableLabel) TappedSecondary(_ *fyne.PointEvent) {
}

type EnterEntry struct {
	widget.Entry
	onEnter func()
}

func NewEnterEntry() *EnterEntry {
	entry := &EnterEntry{}
	entry.ExtendBaseWidget(entry)

	return entry
}

func (e *EnterEntry) Clear() {
	e.Entry.SetText("")
}

func (e *EnterEntry) SetOnEnter(onEnter func()) {
	e.onEnter = onEnter
}

func (e *EnterEntry) TypedKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyReturn:
		e.onEnter()
	default:
		e.Entry.TypedKey(key)
	}
}

func splitTextToLines(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var result []string
	offset := 0

	for {
		var s string
		if len(text[offset:]) >= maxLength {
			s = text[offset : offset+maxLength]
		} else {
			s = text[offset:]
			return append(result, s)
			break
		}
		offset += maxLength
		result = append(result, s)
	}
	return result
}

type MessageObject struct {
	container *fyne.Container
}

func NewMessageObject(username string, text string, tappedUsername func()) *MessageObject {
	messageObj := &MessageObject{}
	mainContainer := fyne.NewContainerWithLayout(layout.NewVBoxLayout())
	msgBody := canvas.NewRectangle(msgBodyColor)
	msgBody.StrokeWidth = 3
	msgBody.StrokeColor = msgStrokeColor

	mainContainer.AddObject(msgBody)
	mainContainer.AddObject(NewTappableLabel(username, tappedUsername))

	lines := splitTextToLines(text, MAX_MSG_TEXT_LINE_LENGTH)
	for _, textLine := range lines {
		mainContainer.AddObject(canvas.NewText(textLine, color.White))
	}

	messageObj.container = mainContainer
	return messageObj
}

func (messageObj *MessageObject) getContainer() *fyne.Container {
	return messageObj.container
}

type MessageList struct {
	container        *fyne.Container
	OnUsernameSelect func(user models.User)
}

func NewMessageList(OnUsernameSelect func(user models.User)) *MessageList {
	list := &MessageList{
		fyne.NewContainerWithLayout(layout.NewVBoxLayout()),
		OnUsernameSelect}
	return list
}

func (list *MessageList) Clear() {
	var objects []fyne.CanvasObject

	list.container.Objects = objects
	list.container.Refresh()
}

func (list *MessageList) AddMessage(msg models.SavedMessage) {
	messageObject := NewMessageObject(msg.User.Username, msg.Text, func() {
		list.OnUsernameSelect(msg.User)
	})
	list.container.AddObject(messageObject.container)
}

func (list *MessageList) SetMessages(messages []models.SavedMessage) {
	for _, msg := range messages {
		list.AddMessage(msg)
	}
}

func (list *MessageList) AddLabel(text string) {
	list.container.AddObject(widget.NewLabel(text))
}

func (list *MessageList) GetContainer() *fyne.Container {
	return list.container
}

func (list *MessageList) Refresh() {
	list.container.Refresh()
}
