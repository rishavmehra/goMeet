package server

import (
	"log"
	"os"

	"github.com/gofiber/fiber"
	"github.com/joho/godotenv"
)

func Run() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	serverEndpoint := os.Getenv("SERVER_ENDPOINT")

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) {
		c.Send("Hello World")
	})
	app.Listen(serverEndpoint)
}
