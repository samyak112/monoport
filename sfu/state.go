package sfu_server

import (
	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/transport"
	"sync"
)

// Signal types for the queue
type offerSignal struct{ sdp webrtc.SessionDescription }
type candidateSignal struct{ candidate webrtc.ICECandidateInit }
type AnswerSignal struct{ SDP webrtc.SessionDescription }

// SFU (Selective Forwarding Unit) holds the global state for all peer connections.
type SFU struct {
	peersLock         sync.RWMutex
	peers             map[string]*PeerConnectionState
	trackLock         sync.RWMutex
	trackLocals       map[string]*webrtc.TrackLocalStaticRTP // Store the concrete type
	config            webrtc.Configuration
	api               *webrtc.API
	signalChannelSend chan *transport.SignalMessage
}

// PeerConnectionState holds the state for a single peer, including its connection and signaling queue.
type PeerConnectionState struct {
	id             string
	peerConnection *webrtc.PeerConnection
	sfu            *SFU // Reference back to the SFU

	// stateLock protects the fields below, ensuring atomic state updates for this peer.
	stateLock             sync.Mutex
	negotiationInProgress bool
	signalQueue           []interface{} // Queue for offers and ICE candidates
}
