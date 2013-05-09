package main

import (
	"sync"
)

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
