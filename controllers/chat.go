package ctr

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/rishavmehra/gomeet/chat"
	"github.com/rishavmehra/gomeet/wrtc"
)

func RoomChat(c *fiber.Ctx) error {
	return c.Render("chat", fiber.Map{}, "") // Todo: frontend
}

func RoomChatWebsocket(c *websocket.Conn) {
	uuid := c.Params("uuid")
	if uuid == "" {
		return
	}

	wrtc.RoomsLock.Lock()
	room := wrtc.Rooms[uuid]
	wrtc.RoomsLock.Unlock()
	if room == nil {
		return
	}
	if room.Hub == nil {
		return
	}

	chat.PeerChatConn(c.Conn, room.Hub)
}
