package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	router   = gin.Default()
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type MessageType int

const (
	ClientConnected MessageType = iota + 1
	ClientDisconnected
	NewMessage
)

type ClientMsg struct {
	Text     string `json:"text"`
	UserName string `json:"username"`
}

type Message struct {
	Type MessageType
	Conn *websocket.Conn
	Text *ClientMsg
}

type Client struct {
	Conn *websocket.Conn
}

type Server struct {
	messages chan Message
	clients  map[string]*Client
}

func NewServer() *Server {
	return &Server{
		messages: make(chan Message, 10),
		clients:  make(map[string]*Client),
	}
}

func (s *Server) Run() {
	for {
		msg := <-s.messages

		switch msg.Type {
		case ClientConnected:
			addr := msg.Conn.RemoteAddr().String()
			log.Println("Got connection from: ", addr)
			s.clients[addr] = &Client{
				Conn: msg.Conn,
			}

		case ClientDisconnected:
			addr := msg.Conn.RemoteAddr().String()
			delete(s.clients, addr)

		case NewMessage:
			addr := msg.Conn.RemoteAddr().String()
			client := s.clients[addr]

			if client != nil {
				for _, c := range s.clients {
					if c.Conn.RemoteAddr().String() != addr {
						c.Conn.WriteJSON(msg.Text)
					}
				}
			}
		}

	}
}

func (s *Server) handleConn(ctx *gin.Context) {
	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	connection := Message{
		Type: ClientConnected,
		Conn: conn,
	}

	s.messages <- connection

	// write message on socket
	for {
		var msg ClientMsg
		err := conn.ReadJSON(&msg)
		// disconnect if error occured
		if err != nil {
			log.Printf("Unable to read message from client %v\n Closing connection...", conn.RemoteAddr())
			disconnection := Message{
				Type: ClientDisconnected,
				Conn: conn,
			}

			s.messages <- disconnection
			return
		}

		// else broadcast the message
		m := Message{
			Type: NewMessage,
			Text: &msg,
			Conn: conn,
		}
		log.Printf("message from %v: %v", conn.RemoteAddr().String(), msg)
		s.messages <- m
	}

}

func main() {
	// Create room
	server := NewServer()
	go server.Run()

	// handle connections
	router.GET("/ws", server.handleConn)

	router.Run()
}
