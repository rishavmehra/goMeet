package chat

import (
	"testing"
	"time"
)

func newTestClient(hub *Hub) *Client {
	return &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
}

func TestHub_RegisterUnregister(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := newTestClient(hub)
	client2 := newTestClient(hub)

	// Register client1
	hub.register <- client1
	time.Sleep(100 * time.Millisecond) // Give some time for the hub to process

	if _, exists := hub.clients[client1]; !exists {
		t.Errorf("Client1 should be registered, but it's not.")
	}

	// Register client2
	hub.register <- client2
	time.Sleep(100 * time.Millisecond)

	if _, exists := hub.clients[client2]; !exists {
		t.Errorf("Client2 should be registered, but it's not.")
	}

	// Unregister client1
	hub.unregister <- client1
	time.Sleep(100 * time.Millisecond)

	if _, exists := hub.clients[client1]; exists {
		t.Errorf("Client1 should be unregistered, but it's still in the hub.")
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newTestClient(hub)
	hub.register <- client
	time.Sleep(100 * time.Millisecond)

	message := []byte("Hello, client!")
	hub.broadcast <- message

	select {
	case msg := <-client.send:
		if string(msg) != "Hello, client!" {
			t.Errorf("Expected message 'Hello, client!', got %s", string(msg))
		}
	case <-time.After(time.Second):
		t.Errorf("Timed out waiting for message to be received by client.")
	}
}

func TestHub_BroadcastToMultipleClients(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client1 := newTestClient(hub)
	client2 := newTestClient(hub)

	hub.register <- client1
	hub.register <- client2
	time.Sleep(100 * time.Millisecond)

	message := []byte("Message to all clients")
	hub.broadcast <- message

	// Check both clients received the message
	for _, client := range []*Client{client1, client2} {
		select {
		case msg := <-client.send:
			if string(msg) != "Message to all clients" {
				t.Errorf("Expected message 'Message to all clients', got %s", string(msg))
			}
		case <-time.After(time.Second):
			t.Errorf("Timed out waiting for message to be received by client.")
		}
	}
}
