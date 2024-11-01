package server

import (
	"log"
	"os"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	ctr "github.com/rishavmehra/gomeet/controllers"
)

func Run() error {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	serverEndpoint := os.Getenv("SERVER_ENDPOINT")

	app := fiber.New()
	app.Get("/", ctr.Home)
	app.Get("/room/create", ctr.RoomCreate)
	app.Get("/room/:uuid", ctr.Room)

	// (New) returns a new `handler func(*Conn)` that upgrades a client to the
	// websocket protocol, you can pass an optional config.
	app.Get("/room/:uuid/ws", websocket.New(ctr.RoomWebSocket, websocket.Config{
		HandshakeTimeout: 12 * time.Second,
	}))
	app.Get("/stream/:suuid", ctr.Stream)
	app.Get("/stream/:suuid/ws", websocket.New(ctx.StreamWebSocket, websocket.Config{
		HandshakeTimeout: 12 * time.Second,
	}))
	return app.Listen(serverEndpoint)
}
)