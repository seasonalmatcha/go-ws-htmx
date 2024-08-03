package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	ID   string
	hub  *Hub
	conn *websocket.Conn

	Send chan []byte
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	id := uuid.New()

	client := &Client{
		ID:   id.String(),
		hub:  hub,
		conn: conn,
		Send: make(chan []byte, 256),
	}

	client.hub.Register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)

	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			_, _ = w.Write(msg)

			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write(msg)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, text, err := c.conn.ReadMessage()
		log.Printf("Value: %v", string(text))

		if err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Printf("Error: %v", err)
			}
			break
		}

		msg := &Message{}

		reader := bytes.NewReader(text)
		decoder := json.NewDecoder(reader)
		err = decoder.Decode(msg)
		if err != nil {
			log.Printf("Error: %v", err)
		}

		c.hub.Broadcast <- &Message{
			ClientID: c.ID,
			Text:     msg.Text,
		}
	}
}
