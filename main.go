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
var key string = "278d425bdf160c739803"
var secret string = "7ad3773142a6692b25b8"

type Event map[string]interface{}

type Client struct {
	socketId string
	buf      chan Event
}

type Channel struct {
	name    string
	members map[string]*Client
	buf     chan Event
}

func NewChannel(name string) (c *Channel) {
	c = &Channel{
		name,
		make(map[string]*Client),
		make(chan Event, 128),
	}
	go c.Run()
	return
}

func (c *Channel) Submit(event Event) {
	c.buf <- event
}

func (c *Channel) Run() {
	for event := range c.buf {
		for _, client := range c.members {
			// TODO: nonblock
			client.buf <- event
		}
	}
}

var channels map[string]*Channel = make(map[string]*Channel)

// TODO: thread safe
// TODO: authorize.
func (client *Client) Subscribe(channelName string) {
	channel, ok := channels[channelName]
	if !ok {
		channel = NewChannel(channelName)
		channels[channelName] = channel
	}
	channel.members[client.socketId] = client
}

// TODO: thread safe
func (client *Client) Unsubscribe(channelName string) {
	_, ok := channels[channelName]
	if ok {
		delete(channels, channelName)
	}
}

func (client *Client) Trigger(event Event) {
	eventName := event["event"].(string)
	if !strings.HasPrefix(eventName, "client-") {
		//TODO: return error
		log.Println("Invalid event prefix")
		return
	}
	channelName := event["channel"].(string)
	channel, ok := channels[channelName]
	if !ok {
		//TODO: return error.
		return
	}
	channel.Submit(event)
}

func GosherServer(ws *websocket.Conn) {
	// TODO: app id etc...
	log.Println("connected")
	defer ws.Close()

	socket_id := strconv.Itoa(rand.Int())
	client := &Client{socket_id, make(chan Event)}
	err := websocket.JSON.Send(ws, Event{
		"event": "pusher:connection_established",
		"data":  Event{"socket_id": socket_id},
	})
	if err != nil {
		log.Println(err)
	} else {
		go func() {
			for event := range client.buf {
				websocket.JSON.Send(ws, event)
			}
		}()
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
}

func main() {
	http.Handle("/app", websocket.Handler(GosherServer))
	err := http.ListenAndServe(":12345", nil)
	if err != nil {
		log.Fatal(err)
	}
}
