package sfu_server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/transport" // Assuming this is your transport package
)

// NewSFU creates and initializes a new SFU instance.
func NewSFU(api *webrtc.API, signalChannel chan *transport.SignalMessage) *SFU {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	return &SFU{
		peers:             make(map[string]*PeerConnectionState),
		trackLocals:       make(map[string]*webrtc.TrackLocalStaticRTP),
		config:            config,
		api:               api,
		signalChannelSend: signalChannel,
	}
}

// dispatchSignal adds a signal to the peer's queue and starts the processing loop if not already running.
func (s *SFU) DispatchSignal(peerID string, signal interface{}) {
	s.peersLock.RLock()
	pcs, ok := s.peers[peerID]
	s.peersLock.RUnlock()

	// CORRECTED: Use a proper type assertion.
	// We should only proceed for an unknown peer if the signal is an offer.
	if !ok {
		if _, isOffer := signal.(offerSignal); !isOffer {
			log.Printf("Received signal for unknown peer %s, ignoring", peerID)
			return
		}
		// The HandleNewPeerOffer function will create the peer.
		// We can't proceed here as `pcs` is nil.
		return
	}

	pcs.stateLock.Lock()
	pcs.signalQueue = append(pcs.signalQueue, signal)
	// If a processing goroutine is not already running for this peer, start one.
	if !pcs.negotiationInProgress {
		pcs.negotiationInProgress = true
		go pcs.processSignalQueue()
	}
	pcs.stateLock.Unlock()
}

// HandleNewPeerOffer is called when a new peer sends an SDP offer.
func (s *SFU) HandleNewPeerOffer(peerID string, offer webrtc.SessionDescription) {
	s.peersLock.Lock()
	pcs, ok := s.peers[peerID]
	if !ok {
		// This is a new peer, create the connection state.
		log.Printf("Handling offer for new peer: %s", peerID)
		peerConnection, err := s.api.NewPeerConnection(s.config)
		if err != nil {
			log.Printf("Failed to create PeerConnection for %s: %v", peerID, err)
			s.peersLock.Unlock()
			return
		}

		pcs = &PeerConnectionState{
			id:             peerID,
			peerConnection: peerConnection,
			sfu:            s,
			signalQueue:    make([]interface{}, 0),
		}
		s.peers[peerID] = pcs
		s.configurePeerConnection(pcs)
	} else {
		fmt.Println("duplicate came")
	}
	s.peersLock.Unlock()

	// Dispatch the offer to the peer's signal queue for processing.
	s.DispatchSignal(peerID, offerSignal{sdp: offer})
}

// HandleIceCandidate is called when a new ICE candidate is received from a peer.
func (s *SFU) HandleIceCandidate(peerID string, candidateStr string) {
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidateStr), &candidate); err != nil {
		log.Printf("[%s] Error unmarshalling ICE candidate: %v", peerID, err)
		return
	}

	// Dispatch the candidate to the peer's signal queue.
	s.DispatchSignal(peerID, candidateSignal{candidate: candidate})
}

// processSignalQueue processes signals for a single peer sequentially.
func (pcs *PeerConnectionState) processSignalQueue() {
	for {
		pcs.stateLock.Lock()
		if len(pcs.signalQueue) == 0 {
			pcs.negotiationInProgress = false
			pcs.stateLock.Unlock()
			return
		}

		signal := pcs.signalQueue[0]
		pcs.signalQueue = pcs.signalQueue[1:]
		pcs.stateLock.Unlock()

		switch s := signal.(type) {
		case offerSignal:
			pcs.handleOffer(s.sdp)
		case AnswerSignal:
			pcs.handleAnswer(s.SDP)
		case candidateSignal:
			pcs.handleCandidate(s.candidate)
		}
	}
}

// handleOffer processes an SDP offer for a peer.
func (pcs *PeerConnectionState) handleOffer(offer webrtc.SessionDescription) {
	log.Printf("[%s] Processing SDP offer", pcs.id)

	if err := pcs.peerConnection.SetRemoteDescription(offer); err != nil {
		log.Printf("[%s] Failed to set remote description: %v", pcs.id, err)
		pcs.sfu.cleanupPeer(pcs.id)
		return
	}

	pcs.sfu.addExistingTracksToPeer(pcs)

	answer, err := pcs.peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Printf("[%s] Failed to create answer: %v", pcs.id, err)
		pcs.sfu.cleanupPeer(pcs.id)
		return
	}

	if err := pcs.peerConnection.SetLocalDescription(answer); err != nil {
		log.Printf("[%s] Failed to set local description: %v", pcs.id, err)
		pcs.sfu.cleanupPeer(pcs.id)
		return
	}

	log.Printf("[%s] SDP Answer created. Sending to client...", pcs.id)
	pcs.sfu.signalChannelSend <- &transport.SignalMessage{
		PeerID: pcs.id,
		Type:   "answer",
		SDP:    answer.SDP,
	}
}

// handleAnswer processes an SDP answer for a peer.
func (pcs *PeerConnectionState) handleAnswer(answer webrtc.SessionDescription) {
	log.Printf("[%s] Processing SDP answer", pcs.id)

	// Set the remote description to the answer provided by the client.
	// This completes the renegotiation initiated by the SFU.
	if err := pcs.peerConnection.SetRemoteDescription(answer); err != nil {
		log.Printf("[%s] Failed to set remote description for answer: %v", pcs.id, err)
		pcs.sfu.cleanupPeer(pcs.id)
		return
	}

	log.Printf("[%s] Remote description (answer) set successfully. Negotiation complete.", pcs.id)
}

// handleCandidate processes an ICE candidate for a peer.
func (pcs *PeerConnectionState) handleCandidate(candidate webrtc.ICECandidateInit) {
	if pcs.peerConnection.RemoteDescription() == nil {
		log.Printf("[%s] Peer connection not ready, requeuing candidate", pcs.id)
		pcs.stateLock.Lock()
		// Add it back to the front of the queue
		pcs.signalQueue = append([]interface{}{candidateSignal{candidate: candidate}}, pcs.signalQueue...)
		pcs.stateLock.Unlock()
		return
	}

	if err := pcs.peerConnection.AddICECandidate(candidate); err != nil {
		log.Printf("[%s] Error adding ICE candidate: %v", pcs.id, err)
	} else {
		log.Printf("[%s] Added ICE candidate from client.", pcs.id)
	}
}

// configurePeerConnection sets up all the necessary callbacks for a new peer connection.
func (s *SFU) configurePeerConnection(pcs *PeerConnectionState) {
	peerID := pcs.id
	peerConnection := pcs.peerConnection

	peerConnection.OnNegotiationNeeded(func() {
		log.Printf("[%s] Negotiation needed, creating new offer...", peerID)
		offer, err := peerConnection.CreateOffer(nil)
		if err != nil {
			log.Printf("[%s] Failed to create negotiation offer: %v", peerID, err)
			return
		}
		if err := peerConnection.SetLocalDescription(offer); err != nil {
			log.Printf("[%s] Failed to set local description for negotiation: %v", peerID, err)
			return
		}
		s.signalChannelSend <- &transport.SignalMessage{
			PeerID: peerID,
			Type:   "offer",
			SDP:    offer.SDP,
		}
	})

	peerConnection.OnTrack(s.handleIncomingTrack(pcs))
	peerConnection.OnICECandidate(s.handleICECandidate(peerID))
	peerConnection.OnConnectionStateChange(s.handleConnectionStateChange(peerID))

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {
		log.Printf("[%s] ICE Connection State has changed: %s", peerID, state.String())
	})
}

// addExistingTracksToPeer adds all currently active tracks to a new peer's connection.
func (s *SFU) addExistingTracksToPeer(pcs *PeerConnectionState) {
	s.trackLock.RLock()
	defer s.trackLock.RUnlock()

	if len(s.trackLocals) == 0 {
		return
	}

	log.Printf("[%s] Adding %d existing tracks to new peer connection", pcs.id, len(s.trackLocals))
	for globalTrackID, localTrack := range s.trackLocals {
		log.Printf("[%s] Adding existing track %s to new peer", pcs.id, globalTrackID)
		if _, err := pcs.peerConnection.AddTrack(localTrack); err != nil {
			log.Printf("[%s] Failed to add existing track %s to new peer: %v", pcs.id, globalTrackID, err)
		}
	}
}

// handleIncomingTrack is called when a remote track is received from a peer.
func (s *SFU) handleIncomingTrack(pcs *PeerConnectionState) func(*webrtc.TrackRemote, *webrtc.RTPReceiver) {
	return func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		fmt.Println("ready to process tracks")
		globalTrackID := fmt.Sprintf("%s_%s_%s", pcs.id, remoteTrack.Kind(), remoteTrack.ID())

		localTrack, err := webrtc.NewTrackLocalStaticRTP(remoteTrack.Codec().RTPCodecCapability, remoteTrack.ID(), remoteTrack.StreamID())
		if err != nil {
			log.Printf("[%s] Failed to create local track for forwarding: %v", pcs.id, err)
			return
		}

		s.trackLock.Lock()
		s.trackLocals[globalTrackID] = localTrack
		s.trackLock.Unlock()

		log.Printf("Created local track %s to forward from peer %s", globalTrackID, pcs.id)
		s.addTrackToPeers(localTrack, globalTrackID, pcs.id)
		go s.forwardRTP(pcs.id, globalTrackID, remoteTrack, localTrack)
	}
}

// addTrackToPeers adds a new local track to all connected peers except the originator.
func (s *SFU) addTrackToPeers(localTrack *webrtc.TrackLocalStaticRTP, globalTrackID, originatorPeerID string) {
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()

	for otherPeerID, otherPCS := range s.peers {
		if otherPeerID == originatorPeerID {
			continue
		}
		if _, err := otherPCS.peerConnection.AddTrack(localTrack); err != nil {
			log.Printf("Failed to add track %s to peer %s: %v", globalTrackID, otherPeerID, err)
		}
	}
}

// forwardRTP reads packets from a remote track and writes them to a local track.
func (s *SFU) forwardRTP(peerID, globalTrackID string, remoteTrack *webrtc.TrackRemote, localTrack *webrtc.TrackLocalStaticRTP) {
	defer func() {
		log.Printf("Finished forwarding for track %s from peer %s.", globalTrackID, peerID)
		s.removeTrack(globalTrackID)
	}()

	rtpBuf := make([]byte, 1500)
	for {
		i, _, readErr := remoteTrack.Read(rtpBuf)
		if readErr != nil {
			if readErr != io.EOF {
				log.Printf("Error reading from remote track %s: %v", globalTrackID, readErr)
			}
			return
		}

		if _, writeErr := localTrack.Write(rtpBuf[:i]); writeErr != nil && writeErr != io.ErrClosedPipe {
			log.Printf("Error writing to local track %s: %v", globalTrackID, writeErr)
			return
		}
	}
}

// removeTrack cleans up a track from the SFU and all peer connections.
func (s *SFU) removeTrack(globalTrackID string) {
	s.trackLock.Lock()
	trackToRemove, ok := s.trackLocals[globalTrackID]
	if !ok {
		s.trackLock.Unlock()
		return
	}
	delete(s.trackLocals, globalTrackID)
	s.trackLock.Unlock()

	log.Printf("Removed track %s from SFU state", globalTrackID)

	s.peersLock.RLock()
	defer s.peersLock.RUnlock()
	for _, pcs := range s.peers {
		for _, sender := range pcs.peerConnection.GetSenders() {
			if sender.Track() != nil && sender.Track().ID() == trackToRemove.ID() {
				if err := pcs.peerConnection.RemoveTrack(sender); err != nil {
					log.Printf("Error removing track %s from peer %s: %v", globalTrackID, pcs.id, err)
				}
			}
		}
	}
}

// handleConnectionStateChange cleans up a peer if the connection fails or closes.
func (s *SFU) handleConnectionStateChange(peerID string) func(webrtc.PeerConnectionState) {
	return func(state webrtc.PeerConnectionState) {
		log.Printf("[%s] Peer Connection State has changed: %s", peerID, state.String())
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			s.cleanupPeer(peerID)
		}
	}
}

// handleICECandidate forwards local ICE candidates to the signaling server.
func (s *SFU) handleICECandidate(peerID string) func(*webrtc.ICECandidate) {
	return func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Printf("[%s] Error marshalling ICE candidate: %v", peerID, err)
			return
		}
		s.signalChannelSend <- &transport.SignalMessage{
			PeerID:    peerID,
			Type:      "candidate",
			Candidate: string(candidateJSON),
		}
	}
}

// cleanupPeer removes a peer and all of its associated resources.
func (s *SFU) cleanupPeer(peerID string) {
	s.peersLock.Lock()
	pcs, ok := s.peers[peerID]
	if !ok {
		s.peersLock.Unlock()
		return
	}
	delete(s.peers, peerID)
	s.peersLock.Unlock()

	if err := pcs.peerConnection.Close(); err != nil {
		log.Printf("[%s] Error closing peer connection: %v", peerID, err)
	}
	log.Printf("Peer %s and its connection removed.", peerID)

	s.trackLock.RLock()
	var tracksToRemove []string
	for globalTrackID := range s.trackLocals {
		if len(globalTrackID) > len(peerID) && globalTrackID[:len(peerID)] == peerID && globalTrackID[len(peerID)] == '_' {
			tracksToRemove = append(tracksToRemove, globalTrackID)
		}
	}
	s.trackLock.RUnlock()

	for _, trackID := range tracksToRemove {
		s.removeTrack(trackID)
	}
	log.Printf("Cleaned up all tracks originated by peer %s.", peerID)
}
