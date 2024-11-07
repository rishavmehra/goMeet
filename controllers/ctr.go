package ctr

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/rishavmehra/gomeet/chat"
	"github.com/rishavmehra/gomeet/wrtc"
)

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

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

	// uuid, stream_uuid, _ := createOrGetRoom(uuid)

	// TODO: Render the Frontend
	return nil // Will work not this
}

func RoomWebSocket(c *websocket.Conn) {
	uuid := c.Params("uuid")
	if uuid == "" {
		return
	}

	// _, _, room := createOrGetRoom(uuid)

}

func createOrGetRoom(uuid string) (string, string, *wrtc.Room) {
	wrtc.RoomsLock.Lock()
	defer wrtc.RoomsLock.Unlock()

	h := sha256.New()
	h.Write([]byte(uuid))
	stream_uuid := fmt.Sprintf("%x", h.Sum(nil))

	//checking for an existig room
	if room := wrtc.Rooms[uuid]; room != nil {
		if _, ok := wrtc.Streams[stream_uuid]; !ok {
			wrtc.Streams[stream_uuid] = room
		}
		return uuid, stream_uuid, room
	}

	// creating new room if its doesnt exist
	hub := chat.NewHub()
	p := &wrtc.Peers{}
	p.TrackLocals = make(map[string]*webrtc.TrackLocalStaticRTP)
	room := &wrtc.Room{
		Peers: p,
		Hub:   hub,
	}

	wrtc.Rooms[uuid] = room
	wrtc.Streams[stream_uuid] = room
	go hub.Run()
	return uuid, stream_uuid, room
}

func peerRoomConn(c *websocket.Conn, p *wrtc.Peers) {

	// create new peerConnection
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
		fmt.Errorf("error in creating peer connection: ", err)
		return
	}
	defer peerConnection.Close()

	// add one audio and one video track incoming
	for _, typ := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		if _, err := peerConnection.AddTransceiverFromKind(typ, webrtc.RTPTransceiverInit{
			Direction: webrtc.RTPTransceiverDirectionRecvonly,
		}); err != nil {
			log.Errorf("failed to add transceiver: %v", err)
			return
		}
	}

	newPeer := wrtc.PeerConnectionState{
		PeerConnection: peerConnection, // created above
		Websocket: &wrtc.ThreadSafeWriter{
			Conn:  c,
			Mutex: sync.Mutex{},
		},
	}

	// add new PeerConnnection to global list
	p.ListLock.Lock()
	p.Connections = append(p.Connections, newPeer)
	p.ListLock.Unlock()

	// trickle ICE Emit server candidate to client
	peerConnection.OnICECandidate(func(i *webrtc.ICECandidate) {
		if i == nil {
			return
		}
		candidateString, err := json.Marshal(i.ToJSON())
		if err != nil {
			log.Errorf("failed to marshal candidate to json: %v", err)
		}
		log.Infof("Send candidate to client: %s\n ")

		if writeErr := newPeer.Websocket.WriteJSON(&websocketMessage{
			Event: "candidate",
			Data:  string(candidateString),
		}); writeErr != nil {
			log.Errorf("failed  to write JSON: %v", writeErr)
		}
	})

	// if peerConnection is close remove it from gloabal list
	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		log.Infof("connection state change: %s\n", pcs)
		switch pcs {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				log.Errorf("Failed to close PeerConnection: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			p.SignalPeerConnections()
		}
	})

	peerConnection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Infof("Got  remote track: kind=%s, ID:%s, PayloadType=%d\n", tr.Kind(), tr.ID(), tr.PayloadType())

		trackLocal := p.AddRemoteTrackToLocal(tr)
		defer p.RemoveLocalTrack(trackLocal)

		buf := make([]byte, 1500)
		rtpPkt := &rtp.Packet{}

		for {
			i, _, err := tr.Read(buf)
			if err != nil {
				return
			}

			if err = rtpPkt.Unmarshal(buf[:i]); err != nil {
				log.Errorf("failed to unmarshall incoming RTP packet: %v", err)
				return
			}

			rtpPkt.Extension = false
			rtpPkt.Extensions = nil

			if err := trackLocal.WriteRTP(rtpPkt); err != nil {
				return
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Infof("ICE connection is changed: %s\n", is)
	})

	// singnal for new peer connection
	p.SignalPeerConnections()

	message := &websocketMessage{}
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Errorf("failed to read message: %v", err)
			return
		}

		log.Infof("Got message: %s", raw)

		if err := json.Unmarshal(raw, &message); err != nil {
			log.Errorf("Failed to unmarshall json to message: %v", err)
			return
		}
		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Errorf("Failed to unmarshall json to candidate: %v", err)
				return
			}
			log.Infof("Got candidate: %v\n", candidate)

			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Errorf("Failed to add ICE to candidate: %v", err)
				return
			}

		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				log.Errorf("Failed to unmarshall json to answer: %v", err)
				return
			}
			log.Infof("Got answer: %v\n", answer)

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Errorf("Failed to set remote Description to candidate: %v", err)
				return
			}
		default:
			log.Errorf("unknown message:", message)

		}
	}
}
