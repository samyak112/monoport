package ws

import (
	"github.com/gorilla/websocket"
	"sync"
)

type Signal struct {
	PeerMap    map[string]*SignalingPeer
	signalLock sync.Mutex
}

type SignalingPeer struct {
	PeerId     string
	Connection *websocket.Conn
}

// SignalMessage is a generic struct for messages to/from signaling server
type SignalMessage struct {
	PeerID    string `json:"peerId,omitempty"`
	Type      string `json:"type"` // "offer", "answer", "candidate"
	SDP       string `json:"sdp,omitempty"`
	Candidate string `json:"candidate,omitempty"` // JSON string of webrtc.ICECandidateInit
}
