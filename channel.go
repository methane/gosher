package main

import (
	"log"
)

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

func (c *Channel) Submit(event Event) {
	c.buf <- event
}
