package main

import (
	"code.google.com/p/go.net/websocket"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
)

// http://pusher.com/docs/auth_signatures
var debug_key string = "278d425bdf160c739803"
var debug_secret string = "7ad3773142a6692b25b8"

type Event map[string]interface{}

type Client struct {
	socketId string
	buf      chan Event
	app      *App
	conn     *websocket.Conn
}

func newClient(app *App, conn *websocket.Conn) *Client {
	socketId := strconv.Itoa(rand.Int())
	log.Println("New client:", socketId)
	client := &Client{
		socketId,
		make(chan Event, 16),
		app,
		conn}
	go func() {
		defer func() {
			conn.Close()
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()

		for event := range client.buf {
			err := websocket.JSON.Send(conn, event)
			if err != nil {
				log.Println(err)
				break
			}
		}
	}()
	return client
}

// Send msg to client withot blocking.
func (client *Client) Send(msg Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Println(r)
		}
	}()
	select {
	case client.buf <- msg:
	default:
		log.Println("Can't send event to", client.socketId, ". Buffer full.")
	}
}

// TODO: thread safe
// TODO: authorize.
func (client *Client) Subscribe(channelName string) {
	client.app.Subscribe(client, channelName)
}

// TODO: thread safe
func (client *Client) Unsubscribe(channelName string) {
	_, ok := client.app.channels[channelName]
	if ok {
		delete(client.app.channels, channelName)
	}
}

// Trigger event from client.
func (client *Client) Trigger(event Event) {
	eventName := event["event"].(string)
	if !strings.HasPrefix(eventName, "client-") {
		//TODO: return error
		log.Println("Invalid event prefix")
		return
	}
	channelName := event["channel"].(string)
	channel, ok := client.app.channels[channelName]
	if !ok {
		//TODO: return error.
		log.Println("Unknown channel:", channelName)
		return
	}
	channel.Submit(event)
}

func GosherServer(ws *websocket.Conn) {
	// TODO: app id etc...
	defer ws.Close()

	app := appRegistory.GetApp("default")
	client := newClient(app, ws)
	client.Send(Event{
		"event": "pusher:connection_established",
		"data":  Event{"socket_id": client.socketId},
	})

	for {
		var event Event
		if err := websocket.JSON.Receive(ws, &event); err != nil {
			log.Println(err)
			break
		}
		log.Println("Received event:", event)
		eventNameX, ok := event["event"]
		if !ok {
			log.Println("Unknown event:", event)
			continue
		}
		eventName, ok := eventNameX.(string)
		if !ok {
			log.Println("Unknown event:", event)
			continue
		}
		switch {
		case eventName == "pusher:subscribe":
			client.Subscribe(event["data"].(map[string]interface{})["channel"].(string))
		case eventName == "pusher:unsubscribe":
			client.Unsubscribe(event["data"].(map[string]interface{})["channel"].(string))
		case strings.HasPrefix(eventName, "client-"):
			client.Trigger(event)
		default:
			log.Println("Unknown event:", event)
		}
	}
}

func main() {
	http.Handle("/app", websocket.Handler(GosherServer))
	err := http.ListenAndServe(":12345", nil)
	if err != nil {
		log.Fatal(err)
	}
}
