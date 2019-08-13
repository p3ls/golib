// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package golib

import (
    // "bytes"
    "log"
    "net/http"
    "time"
    "github.com/gorilla/websocket"
)

const (
    // Time allowed to write a message to the peer.
    writeWait = 10 * time.Second

    // Time allowed to read the next pong message from the peer.
    pongWait = 60 * time.Second

    // Send pings to peer with this period. Must be less than pongWait.
    pingPeriod = (pongWait * 9) / 10

    // Maximum message size allowed from peer.
    maxMessageSize = 512
)

var (
    newline = []byte{'\n'}
    space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
    CheckOrigin: devCheckOrigin,
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

// hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
    // Registered clients.
    clients map[*Client]bool

    // Inbound messages from the clients.
    broadcast chan []byte

    // Register requests from the clients.
    register chan *Client

    // Unregister requests from clients.
    unregister chan *Client
}

func NewWebsocket() *Hub {
    return &Hub{
        broadcast:  make(chan []byte),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        clients:    make(map[*Client]bool),
    }
}

// Websocket Message Types

// const (
//     // TextMessage denotes a text data message. The text message payload is
//     // interpreted as UTF-8 encoded text data.
//     TextMessage = 1

//     // BinaryMessage denotes a binary data message.
//     BinaryMessage = 2

//     // CloseMessage denotes a close control message. The optional message
//     // payload contains a numeric code and text. Use the FormatCloseMessage
//     // function to format a close message payload.
//     CloseMessage = 8

//     // PingMessage denotes a ping control message. The optional message payload
//     // is UTF-8 encoded text.
//     PingMessage = 9

//     // PongMessage denotes a pong control message. The optional message payload
//     // is UTF-8 encoded text.
//     PongMessage = 10
// )

func (h *Hub) Send(msg []byte) {
    for client := range h.clients {
        client.send <- msg
    }
}

func (h *Hub) Run() {
    // log.Println("websocket running...")
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
            }
        case message := <-h.broadcast:
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
        }
    }
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
    hub *Hub

    // The websocket connection.
    conn *websocket.Conn

    // Buffered channel of outbound messages.
    send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("error: %v", err)
            }
            break
        }
        // message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
        c.hub.broadcast <- message
    }
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                // The hub closed the channel.
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }

            w, err := c.conn.NextWriter(websocket.TextMessage)
            if err != nil {
                return
            }
            w.Write(message)

            // // Add queued chat messages to the current websocket message.
            // n := len(c.send)
            // for i := 0; i < n; i++ {
            //     w.Write(newline)
            //     w.Write(<-c.send)
            // }

            if err := w.Close(); err != nil {
                return
            }
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}

func devCheckOrigin(r *http.Request) bool { 
    return true 
}


// ServeWebsocket handles websocket requests from the peer.
func ServeWebsocket(hub *Hub, w http.ResponseWriter, r *http.Request) error {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
        return err
    }
    
    // client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
    client := &Client{hub: hub, conn: conn, send: make(chan []byte)}
    client.hub.register <- client

    // Allow collection of memory referenced by the caller by doing all work in
    // new goroutines.
    go client.writePump()
    go client.readPump()

    return nil
}
