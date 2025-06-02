package sfu_server

import (
	"github.com/pion/webrtc/v3"
	"sync"
)

// SFU holds the global state for the Selective Forwarding Unit
type SFU struct {
	peers       map[string]*PeerConnectionState // Map of peerID to their connection state
	trackLocals map[string]webrtc.TrackLocal    // Map of globalTrackID to TrackLocalStaticRTP
	// globalTrackID is typically peerID + track.Kind() + track.ID() to ensure uniqueness
	lock          sync.RWMutex
	config        webrtc.Configuration
	api           *webrtc.API
	signalChannel chan SignalMessage // A channel to simulate sending messages to signaling server
}

// PeerConnectionState holds the state for a single peer connection
type PeerConnectionState struct {
	peerConnection *webrtc.PeerConnection
	id             string // Unique ID for this peer
}

// SignalMessage is a generic struct for messages to/from signaling server
type SignalMessage struct {
	PeerID    string `json:"peerId"`
	Type      string `json:"type"` // "offer", "answer", "candidate"
	SDP       string `json:"sdp,omitempty"`
	Candidate string `json:"candidate,omitempty"` // JSON string of webrtc.ICECandidateInit
}
