package ctr

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"

	"github.com/pion/rtp"
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
		streamConnection(c, stream.Peers)
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

	peerConnection.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) {
		log.Infof("Connection state change: %s\n", pcs)
		switch pcs {
		case webrtc.PeerConnectionStateFailed:
			if err := peerConnection.Close(); err != nil {
				log.Errorf("failed to close peer connection: %v", err)
			}
		case webrtc.PeerConnectionStateClosed:
			p.SignalPeerConnections()
		}
	})

	peerConnection.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Infof("Got remote track: Kind=%s, ID=%s, PayloadType=%d\n", tr.Kind(), tr.ID(), tr.PayloadType())

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
				log.Errorf("failed to unmarshall incomming RTP Packet: %v", err)
				return
			}

			rtpPkt.Extension = false
			rtpPkt.Extensions = nil

			if err = trackLocal.WriteRTP(rtpPkt); err != nil {
				return
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) {
		log.Infof("ice connection state chaneged: %s\n", is)
	})

	p.SignalPeerConnections()

	message := &websocketMessage{}

	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			log.Errorf("Failed to read message: %v", err)
		}
		log.Infof("Got message: %s", raw)

		if err = json.Unmarshal(raw, &message); err != nil {
			log.Errorf("Failed to unmarshall json to message: %v", err)
			return
		}

		switch message.Event {
		case "candidate":
			candidate := webrtc.ICECandidateInit{}
			if err := json.Unmarshal([]byte(message.Data), &candidate); err != nil {
				log.Errorf("failed to unmarshall json to candidate: %v", err)
				return
			}

			log.Infof("Got candidate: %v\n", candidate)

			if err := peerConnection.AddICECandidate(candidate); err != nil {
				log.Errorf("Failed to add ICE candidate: %v", err)
				return
			}
		case "answer":
			answer := webrtc.SessionDescription{}
			if err := json.Unmarshal([]byte(message.Data), &answer); err != nil {
				log.Errorf("failed to unmarshall json to answer: %v", err)
				return
			}

			log.Infof("Got answer: %v\n", answer)

			if err := peerConnection.SetRemoteDescription(answer); err != nil {
				log.Errorf("Failed to set remote description: %v", err)
				return
			}
		default:
			log.Errorf("unknown message: %+v", message)
		}
	}

}

func StreamViewerWebsocket(c *websocket.Conn) {
	stream_uuid := c.Params("stream_uuid")
	if stream_uuid == "" {
		return
	}

	wrtc.RoomsLock.Lock()
	if peer, ok := wrtc.Rooms[stream_uuid]; ok {
		wrtc.RoomsLock.Unlock()
		streamViwerConn(c, peer.Peers)
		return
	}
	wrtc.RoomsLock.Unlock()

}

func streamViwerConn(c *websocket.Conn, p *wrtc.Peers) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer c.Close()

	for {
		select {
		case <-ticker.C:
			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write([]byte(fmt.Sprintf("%d", len(p.Connections))))
		}
	}
}
