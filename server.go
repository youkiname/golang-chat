// server.go
package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

func getUser(username string, passwordHash string) (User, error) {
	if username == "vadim" && passwordHash == "202cb962ac59075b964b07152d234b70" {
		return User{1, "vadim", passwordHash}, nil
	}
	if username == "q" && passwordHash == "7694f4a66316e53c8cdd9d9954bd611d" {
		return User{2, "q", passwordHash}, nil
	}
	return User{}, errors.New("Username or password is invalid.")
}

func main() {
	host := "localhost:3811"
	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("Connected " + c.Id())
		c.Join("main")
	})
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		log.Println("Disconnected " + c.Id())
	})

	server.On("/login", func(c *gosocketio.Channel, data LoginData) {
		time.Sleep(1 * time.Second)
		user, err := getUser(data.Username, data.PasswordHash)
		if err == nil {
			c.Emit("/login", user)
		}
	})

	server.On("/message", func(c *gosocketio.Channel, msg Message) {
		_, err := getUser(msg.User.Username, msg.User.PasswordHash)
		if err == nil {
			c.BroadcastTo("main", "/message", msg)
		}
	})

	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", server)

	log.Println("Starting server...")
	log.Panic(http.ListenAndServe(host, serveMux))
}
