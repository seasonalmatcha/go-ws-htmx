package main

import (
	"bytes"
	"log"
	"sync"
	"text/template"
)

type Hub struct {
	sync.RWMutex

	Clients map[*Client]bool

	Broadcast  chan *Message
	Register   chan *Client
	Unregister chan *Client

	Messages []*Message
}

func NewHub() *Hub {
	return &Hub{
		Clients:    map[*Client]bool{},
		Broadcast:  make(chan *Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Lock()
			h.Clients[client] = true
			h.Unlock()

			log.Printf("Client registered: %s", client.ID)

			for _, msg := range h.Messages {
				client.Send <- getMessageTemplate(msg)
			}
		case client := <-h.Unregister:
			h.Lock()
			if _, ok := h.Clients[client]; ok {
				close(client.Send)
				log.Printf("Client unregistered: %s", client.ID)
				delete(h.Clients, client)
			}
			h.Unlock()
		case msg := <-h.Broadcast:
			h.RLock()
			h.Messages = append(h.Messages, msg)

			for client := range h.Clients {
				select {
				case client.Send <- getMessageTemplate(msg):
				default:
					close(client.Send)
					delete(h.Clients, client)
				}
			}
			h.RUnlock()
		}
	}
}

func getMessageTemplate(msg *Message) []byte {
	tmpl, err := template.ParseFiles("templates/message.html")
	if err != nil {
		log.Fatalf("Template parsing error: %s", err)
	}

	var renderedMessage bytes.Buffer
	err = tmpl.Execute(&renderedMessage, msg)
	if err != nil {
		log.Fatalf("Template execution error: %s", err)
	}

	return renderedMessage.Bytes()
}
