// server.go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

func main() {
	host := "localhost:3811"
	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	db := DatabaseAdapter{}
	db.connectSqlite("app.db")
	defer db.Close()

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("Connected " + c.Id())
	})
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		log.Println("Disconnected " + c.Id())
	})

	server.On("/login", func(c *gosocketio.Channel, data LoginData) {
		time.Sleep(1 * time.Second)
		user, err := db.getUserByName(data.Username)
		if err == nil && user.PasswordHash == data.PasswordHash {
			log.Println("New login " + user.Username)
			c.Join("main")
			c.Emit("/login", user)
		} else {
			log.Println("Failed login " + user.Username)
			c.Emit("/failed-login", LoginError{"Username or password is not correct."})
		}
	})

	server.On("/register", func(c *gosocketio.Channel, data LoginData) {
		if db.isUserExist(data.Username) {
			c.Emit("/failed-registeration",
				LoginError{"Username " + data.Username + " already exists."})
		} else {
			user := User{Username: data.Username, PasswordHash: data.PasswordHash}
			db.addNewUser(&user)
			c.Join("main")
			c.Emit("/login", user)
		}
	})
	server.On("/message", func(c *gosocketio.Channel, msg Message) {
		if db.checkUserPassword(msg.User.Username, msg.User.PasswordHash) {
			savedMessage := db.addNewMessage(msg)
			c.BroadcastTo("main", "/message", savedMessage)
		}
	})
	server.On("/last-messages", func(c *gosocketio.Channel, chatId int64, user User) {
		messages := db.getMessagesFromChat(user.Id, chatId)
		for _, msg := range messages {
			log.Println(msg.UserData.Username + ": " + msg.Text)
		}
		c.Emit("/last-messages", messages)
	})

	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", server)

	log.Println("Starting server...")
	log.Panic(http.ListenAndServe(host, serveMux))
}
