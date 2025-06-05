package sfu_server

import (
	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/transport"
	"sync"
)

// SFU holds the global state for the Selective Forwarding Unit
type SFU struct {
	peers       map[string]*PeerConnectionState // Map of peerID to their connection state
	trackLocals map[string]webrtc.TrackLocal    // Map of globalTrackID to TrackLocalStaticRTP
	// globalTrackID is typically peerID + track.Kind() + track.ID() to ensure uniqueness
	lock              sync.RWMutex
	config            webrtc.Configuration
	api               *webrtc.API
	signalChannelSend chan *transport.SignalMessage // A channel to simulate sending messages to signaling server
}

// PeerConnectionState holds the state for a single peer connection
type PeerConnectionState struct {
	peerConnection *webrtc.PeerConnection
	id             string // Unique ID for this peer
}
