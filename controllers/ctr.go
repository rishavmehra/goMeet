package ctr

import (
	"fmt"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func Home(c *fiber.Ctx) error {
	return c.SendString("Hello World")
}

func RoomCreate(c *fiber.Ctx) error {
	return c.Redirect(fmt.Sprintf("/room/%s", uuid.New().String()))
}

func Room(c *fiber.Ctx) error {
	// Params is used to get the route parameters
	uuid := c.Params("uuid")
	if uuid == "" {
		c.Status(400)
		return nil
	}
}

func RoomWebSocket(c *websocket.Conn) {
	uuid := c.Params("uuid")
	if uuid == "" {
		return
	}
}

func createOrGetRoom(uuid string) (string, string, *w.Room)
