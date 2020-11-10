// custom_widgets
package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

var separatorColor = color.RGBA{33, 150, 243, 255}
var msgBodyColor = color.RGBA{125, 119, 119, 255}
var msgStrokeColor = color.RGBA{80, 80, 80, 255}

const MAX_MSG_TEXT_LINE_LENGTH int = 70

type tappableLabel struct {
	widget.Label
	TappedFunc func()
}

func newTappableLabel(text string, tappedFunc func()) *tappableLabel {
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

func newMessageObject(username string, text string, tappedUsername func()) *MessageObject {
	messageObj := &MessageObject{}
	mainContainer := fyne.NewContainerWithLayout(layout.NewVBoxLayout())
	msgBody := canvas.NewRectangle(msgBodyColor)
	msgBody.StrokeWidth = 3
	msgBody.StrokeColor = msgStrokeColor

	mainContainer.AddObject(msgBody)
	mainContainer.AddObject(newTappableLabel(username, tappedUsername))

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
	OnUsernameSelect func(user UserPublicInfo)
}

func newMessageList(OnUsernameSelect func(user UserPublicInfo)) *MessageList {
	list := &MessageList{
		fyne.NewContainerWithLayout(layout.NewVBoxLayout()),
		OnUsernameSelect}
	return list
}

func (c *MessageList) clear() {
	var objects []fyne.CanvasObject

	c.container.Objects = objects
	c.container.Refresh()
}

func (c *MessageList) addMessage(msg SavedMessage) {
	messageObject := newMessageObject(msg.UserData.Username, msg.Text, func() {
		c.OnUsernameSelect(msg.UserData)
	})
	c.container.AddObject(messageObject.container)
}

func (c *MessageList) setMessages(messages []SavedMessage) {
	for _, msg := range messages {
		c.addMessage(msg)
	}
}

func (c *MessageList) addLabel(text string) {
	c.container.AddObject(widget.NewLabel(text))
}

func (c *MessageList) getContainer() *fyne.Container {
	return c.container
}

func (c *MessageList) refresh() {
	c.container.Refresh()
}
