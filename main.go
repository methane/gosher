package main

import (
	"code.google.com/p/go.net/websocket"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// http://pusher.com/docs/auth_signatures
var debug_key string = "278d425bdf160c739803"
var debug_secret string = "7ad3773142a6692b25b8"

type Event map[string]interface{}

type App struct {
	name     string
	key      string
	secret   string
	channels map[string]*Channel
	mutex    *sync.Mutex
}

func newApp(name string) *App {
	return &App{
		name,
		debug_key,
		debug_secret,
		make(map[string]*Channel),
		&sync.Mutex{},
	}
}

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

type AppRegistory struct {
	apps  map[string]*App
	mutex *sync.Mutex
}

func newAppRegistory() *AppRegistory {
	return &AppRegistory{
		make(map[string]*App),
		&sync.Mutex{},
	}
}

var appRegistory *AppRegistory = newAppRegistory()

func (registory *AppRegistory) GetApp(name string) *App {
	registory.mutex.Lock()
	defer registory.mutex.Unlock()
	app, ok := registory.apps[name]
	if !ok {
		app = newApp(name)
		registory.apps[name] = app
	}
	return app
}

type Channel struct {
	name      string
	members   map[string]*Client
	buf       chan Event
	subscribe chan *Client
}

func newChannel(name string) *Channel {
	c := &Channel{
		name,
		make(map[string]*Client),
		make(chan Event, 16),
		make(chan *Client),
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println(r)
			}
		}()
		for {
			select {
			case event, ok := <-c.buf:
				if ok {
					for _, client := range c.members {
						client.Send(event)
					}
				} else {
					break
				}
			case client := <-c.subscribe:
				c.members[client.socketId] = client
			}
		}
	}()
	return c
}

func (app *App) GetChannel(name string) *Channel {
	app.mutex.Lock()
	defer app.mutex.Unlock()
	c, _ := app.channels[name]
	return c
}

func (app *App) Subscribe(client *Client, channelName string) {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	channel, ok := app.channels[channelName]
	if !ok {
		channel = newChannel(channelName)
		app.channels[channelName] = channel
	}
	channel.subscribe <- client
}

func (c *Channel) Submit(event Event) {
	c.buf <- event
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
