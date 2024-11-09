package ctr

import (
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	// "github.com/gofiber/fiber/v2/log"
	"github.com/pion/webrtc/v3"
	"github.com/rishavmehra/gomeet/wrtc"
)

func Stream(c *fiber.Ctx) error {
	stream_uuid := c.Params("stream_uuid")
	if stream_uuid == "" {
		c.Status(400)
		return nil
	}

	// render the frontend

}

func StreamWebSocket(c *websocket.Conn) {
	stream_uuid := c.Params("stream_uuid")
	if stream_uuid == "" {
		return
	}

	wrtc.RoomsLock.Lock()
	if stream, ok := wrtc.Streams[stream_uuid]; ok {
		wrtc.RoomsLock.Unlock()
		// streamConnection()
		return
	}
	wrtc.RoomsLock.Unlock()
}

func streamConnection(c *websocket.Conn, p *wrtc.Peers) {
	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
				},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer peerConnection.Close()

	//adding media -> audio and video
	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Errorf("failed to add transceiver: %v", err)
		}
	}

	newPeer := wrtc.PeerConnectionState{
		PeerConnection: peerConnection,
		Websocket: &wrtc.ThreadSafeWriter{
			Conn:  c,
			Mutex: sync.Mutex{},
		},
	}

	wrtc.RoomsLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	wrtc.RoomsLock.Unlock()

	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Errorf("failed to marshal the candidate to json: %v", err)
		}
		log.Infof("send candidate to client: %s\n", candidateString)

		if WriteErr := c.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); WriteErr != nil {
			log.Errorf("failed to write JSON: %v", WriteErr)
		}

	})

}
