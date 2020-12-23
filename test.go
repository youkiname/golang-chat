// test.go
package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"

	"time"

	"github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	"github.com/satori/go.uuid"

	"chat/encrypt"
	"chat/models"
	"chat/utils"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var key uuid.UUID

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func main() {
	reader := bufio.NewReader(os.Stdin)

	key = uuid.FromBytesOrNil([]byte(utils.COMMON_SECRET_KEY))
	host, port := "localhost", 3811
	clients := make(map[int]*gosocketio.Client)

	const maxClients = 1000

	var loginTime [maxClients]int64

	for x := 0; x < maxClients; x++ {
		client, err := gosocketio.Dial(
			gosocketio.GetUrl(host, port, false),
			transport.GetDefaultWebsocketTransport())
		fmt.Printf("Client %d was created\n", x)
		if err != nil {
			log.Panic(err)
		}

		client.On(gosocketio.OnDisconnection, func(h *gosocketio.Channel) {
			log.Println("Disconnected! " + client.Id())
		})

		client.On(gosocketio.OnConnection, func(h *gosocketio.Channel) {
			log.Println("Connected " + client.Id())
		})
		k := x
		client.On("/login", func(h *gosocketio.Channel, encryptedAuthData string) {
			loginTime[k] = time.Now().UnixNano() - loginTime[k]
			if k%10 == 0 {
				fmt.Printf("Estimated time for %d ", k)
				fmt.Print(loginTime[k] / 10000000)
				fmt.Println(" ms")
			}

		})

		clients[x] = client
	}
	authData := models.AuthRequest{"q", encrypt.GetPasswordHash("q")}
	encryptedData := encrypt.Encrypt(key, authData)

	command, err := reader.ReadString('\n')
	command = command[:len(command)-2]
	for err == nil && command != "q" {
		if command == "l" {
			i := 0
			for i < maxClients {
				client := clients[i]

				loginTime[i] = time.Now().UnixNano()
				client.Emit("/login", encryptedData)
				i++
			}
			fmt.Printf("%d sent login try\n", i)
		} else if command == "r" {
			sum := 0.0
			for j := 0; j < maxClients; j++ {
				sum += float64(loginTime[j]) / 10000000.0
			}
			fmt.Printf("Average est. time for login: %f ms", sum/maxClients)
		}
		command, err = reader.ReadString('\n')
		command = command[:len(command)-2]
	}
	if err != nil {
		log.Println(err)
	}
}
