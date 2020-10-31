// custom_widgets
package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
)

var grayColor = color.RGBA{85, 165, 34, 255}

type MessageList struct {
	container *fyne.Container
}

func newMessageList() *MessageList {
	list := &MessageList{}
	list.container = fyne.NewContainerWithLayout(layout.NewVBoxLayout())
	return list
}

func (c *MessageList) clear() {
	var objects []fyne.CanvasObject

	c.container.Objects = objects
	c.container.Refresh()
}

func (c *MessageList) addMessage(msg SavedMessage) {
	maxLength := 70
	if len(msg.Text) < maxLength {
		messageString := msg.UserData.Username + ": " + msg.Text
		c.container.AddObject(canvas.NewText(messageString, color.White))
	} else {
		messageString := msg.UserData.Username + ": " + msg.Text[0:maxLength]
		c.container.AddObject(canvas.NewText(messageString, color.White))

		text := msg.Text
		offset := maxLength

		for {
			var s string
			if len(text[offset:]) >= maxLength {
				s = text[offset : offset+maxLength]
			} else {
				s = text[offset:]
				c.container.AddObject(canvas.NewText(s, color.White))
				break
			}
			offset += maxLength
			c.container.AddObject(canvas.NewText(s, color.White))
		}
	}
	c.container.AddObject(canvas.NewLine(grayColor))
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
