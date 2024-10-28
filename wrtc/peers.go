package wrtc

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/rishavmehra/gomeet/chat"
)

type Room struct {
	Peers *Peers
	Hub   *chat.Hub
}

type Peers struct {
	ListLock    sync.RWMutex
	Connections []PeerConnectionState
	TrackLocals map[string]*webrtc.TrackLocalStaticRTP // have -> trackLocal ID,
}

type PeerConnectionState struct {
	PeerConnection *webrtc.PeerConnection
	Websocket      *ThreadSafeWriter
}

type ThreadSafeWriter struct {
	Conn  *websocket.Conn
	Mutex sync.Mutex
}

func (t *ThreadSafeWriter) WriteJSON(v interface{}) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	return t.Conn.WriteJSON(v)
}

// facilitates adding a media track to the peer's internal state,
// creating a local equivalent of a remote track to maintain a consistent media stream across peer connections.
func (p *Peers) AddRemoteTrackToLocal(t *webrtc.TrackRemote) *webrtc.TrackLocalStaticRTP {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
	}()

	trackLocal, err := webrtc.NewTrackLocalStaticRTP(t.Codec().RTPCodecCapability, t.ID(), t.StreamID())
	if err != nil {
		log.Println(err.Error())
	}

	p.TrackLocals[t.ID()] = trackLocal
	return trackLocal
}

// basically removes the track
func (p *Peers) RemoveLocalTrack(t *webrtc.TrackRemote) {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
	}()
	delete(p.TrackLocals, t.ID())
}

func (p *Peers) SendKeyFrameToConnections() {
	p.ListLock.Lock()
	defer p.ListLock.Unlock()

	for i := range p.Connections {
		for _, receiver := range p.Connections[i].PeerConnection.GetReceivers() {
			if receiver.Track() == nil {
				continue
			}

			_ = p.Connections[i].PeerConnection.WriteRTCP([]rtcp.Packet{
				&rtcp.PictureLossIndication{
					MediaSSRC: uint32(receiver.Track().SSRC()), // SSRC -> Synchronization Source
				},
			})

		}
	}
}

func (p *Peers) SignalPeerConnections() {
	p.ListLock.Lock()
	defer func() {
		p.ListLock.Unlock()
		p.SendKeyFrameToConnections()
	}()

	attemptSync := func() (tryAgain bool) {
		for i := range p.Connections {
			// check for closed connection and remove them
			if p.Connections[i].PeerConnection.ConnectionState() == webrtc.PeerConnectionStateClosed {
				p.Connections = append(p.Connections[:i], p.Connections[i+1:]...)
				log.Println("a", p.Connections)
				return true
			}

			// verifies the current active media tracks and removes any that are not in the list of required local tracks
			existingSenders := map[string]bool{}
			for _, sender := range p.Connections[i].PeerConnection.GetSenders() {
				if sender.Track() == nil {
					continue
				}

				existingSenders[sender.Track().ID()] = true
				// remove unnecessary tracks
				if _, ok := p.TrackLocals[sender.Track().ID()]; !ok {
					if err := p.Connections[i].PeerConnection.RemoveTrack(sender); err != nil {
						return true
					}
				}
			}

			for _, receiver := range p.Connections[i].PeerConnection.GetReceivers() {
				if receiver.Track() == nil {
					continue
				}
				existingSenders[receiver.Track().ID()] = true
			}

			for trackID := range p.TrackLocals {
				if _, ok := existingSenders[trackID]; !ok {
					if _, err := p.Connections[i].PeerConnection.AddTrack(p.TrackLocals[trackID]); err != nil {
						return true
					}
				}
			}
			offer, err := p.Connections[i].PeerConnection.CreateOffer(nil)
			if err != nil {
				return true
			}

			if err = p.Connections[i].PeerConnection.SetLocalDescription(offer); err != nil {
				return true
			}

			offerString, err := json.Marshal(offer)
			if err != nil {
				return true
			}

			if err = p.Connections[i].Websocket.WriteJSON(&websocketMessage{
				Event: "offer",
				Data:  string(offerString),
			}); err != nil {
				return true
			}
		}
		return
	}

	for syncAttempt := 0; ; syncAttempt++ {
		if syncAttempt == 25 {
			go func() {
				time.Sleep(time.Second * 3)
				p.SignalPeerConnections()
			}()
			return
		}
		if !attemptSync() {
			break
		}
	}
}

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}
