package chat

import "github.com/fasthttp/websocket"

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}
