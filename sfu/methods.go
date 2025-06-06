package sfu_server

import (
	"encoding/json"
	"fmt"
	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/transport"
	"io"
	"log"
)

// NewSFU creates and initializes a new SFU instance
func NewSFU(api *webrtc.API, signalChannel chan *transport.SignalMessage) *SFU {

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"}, // Example STUN server
			},
			// You can add TURN servers here if needed:
			// {
			// URLs: []string{"turn:your.turn.server:3478"},
			// Username: "user",
			// Credential: "password",
			// },
		},
	}
	//No need to initialize Mutex here because a struct initializes its elements with a zero values if not given
	// and zero value of a Mutex is an unlocked mutex
	return &SFU{
		peers:             make(map[string]*PeerConnectionState),
		trackLocals:       make(map[string]webrtc.TrackLocal),
		config:            config,
		api:               api,
		signalChannelSend: signalChannel, // Buffered channel for outgoing signals
	}
}

func (s *SFU) HandleIceCandidate(peerId string, candidate string) {

	s.lock.RLock() // Read lock to access s.peers
	pcs, ok := s.peers[peerId]
	s.lock.RUnlock()

	if !ok {
		log.Printf("Received candidate for unknown or not-yet-processed peer %s", peerId)
		return
	}

	var iceCandidateInit webrtc.ICECandidateInit
	if err := json.Unmarshal([]byte(candidate), &iceCandidateInit); err != nil {
		log.Printf("[%s] Error unmarshalling ICE candidate from signal: %v. Data: %s", peerId, err, candidate)
		return
	}

	log.Println("adding the new ice candidate from client")
	if err := pcs.peerConnection.AddICECandidate(iceCandidateInit); err != nil {
		log.Printf("[%s] Error adding ICE candidate: %v", peerId, err)
	} else {
		log.Printf("[%s] Added ICE candidate from signal: %s", peerId, iceCandidateInit.Candidate)
	}
}

// handleNewPeerOffer is called when a new peer sends an SDP offer.
func (s *SFU) HandleNewPeerOffer(peerID string, offer webrtc.SessionDescription) {
	s.lock.Lock() // Full lock for initial peer setup phase
	_, ok := s.peers[peerID]
	if ok {
		delete(s.peers, peerID)
		log.Printf("Peer %s already connected or processing. so replaced it", peerID)
		// return
	}

	s.lock.Unlock()
	s.lock.Lock()
	// log.Printf("Handling offer for new peer: %s", peerID)

	// Create a new PeerConnection for this peer
	peerConnection, err := s.api.NewPeerConnection(s.config)
	if err != nil {
		s.lock.Unlock()
		log.Printf("Failed to create PeerConnection for %s: %v", peerID, err)
		return
	}

	pcs := &PeerConnectionState{
		peerConnection: peerConnection,
		id:             peerID,
	}
	s.peers[peerID] = pcs
	s.lock.Unlock() // Unlock after adding to peers map, specific handlers will use their own locks

	// --- Configure PeerConnection Callbacks ---

	peerConnection.OnNegotiationNeeded(func() {
		fmt.Println("negotiation needed")
	})
	// Set up handling for incoming tracks from this new peer
	peerConnection.OnTrack(func(incomingTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Println("came here for received track")
		trackDetails := fmt.Sprintf("ID=%s, Kind=%s, StreamID=%s, SSRC=%d, Codec=%s",
			incomingTrack.ID(), incomingTrack.Kind(), incomingTrack.StreamID(), incomingTrack.SSRC(), incomingTrack.Codec().MimeType)
		log.Printf("[%s] Track received: %s", peerID, trackDetails)

		// Create a unique global ID for this track within the SFU
		globalTrackID := fmt.Sprintf("%s_%s_%s_%d", peerID, incomingTrack.Kind(), incomingTrack.ID(), incomingTrack.SSRC())

		s.lock.Lock() // Lock for modifying s.trackLocals
		// Avoid duplicate processing if OnTrack is called multiple times for the same track
		if _, ok := s.trackLocals[globalTrackID]; ok {
			s.lock.Unlock()
			log.Printf("[%s] Track %s (globalID: %s) already being forwarded.", peerID, incomingTrack.ID(), globalTrackID)
			return
		}

		// Create a new TrackLocalStaticRTP to forward this track to other peers.
		// This local track will mimic the properties of the incoming track.
		localTrack, newTrackErr := webrtc.NewTrackLocalStaticRTP(
			incomingTrack.Codec().RTPCodecCapability,
			incomingTrack.ID(),       // Use original track ID for the local track
			incomingTrack.StreamID(), // Use original stream ID for the local track
		)
		if newTrackErr != nil {
			s.lock.Unlock()
			log.Printf("[%s] Failed to create TrackLocalStaticRTP for track %s: %v", peerID, incomingTrack.ID(), newTrackErr)
			return
		}
		s.trackLocals[globalTrackID] = localTrack
		s.lock.Unlock() // Unlock after modifying s.trackLocals

		log.Printf("[%s] Created local track (globalID: %s) to forward remote track %s", peerID, globalTrackID, incomingTrack.ID())

		// Forward this new localTrack to all *other* currently connected peers
		s.lock.RLock() // Read lock for iterating s.peers

		log.Println("reached here?")
		for otherPeerID, otherPCS := range s.peers {
			if otherPeerID == peerID { // Don't send track back to the originator
				continue
			}
			log.Printf("Attempting to add track %s (from %s, globalID: %s) to peer %s", localTrack.ID(), peerID, globalTrackID, otherPeerID)
			if _, addTrackErr := otherPCS.peerConnection.AddTrack(localTrack); addTrackErr != nil {
				log.Printf("Failed to add track %s to peer %s: %v", localTrack.ID(), otherPeerID, addTrackErr)
				// Note: Adding tracks dynamically to an existing peer might require renegotiation (new offer/answer)
				// with that 'otherPeerID'. For simplicity, this SFU assumes clients can handle new tracks
				// (which often triggers 'ontrack' on the client side without full SDP exchange if transceivers are set up).
			} else {
				log.Printf("Successfully added track %s (from %s) to peer %s", localTrack.ID(), peerID, otherPeerID)
			}
		}
		s.lock.RUnlock()

		// Start a goroutine to read RTP packets from the incoming remote track
		// and write them to the local track that is being forwarded.
		go func() {
			rtpBuf := make([]byte, 1500) // Standard MTU for Ethernet
			for {
				i, _, readErr := incomingTrack.Read(rtpBuf)
				if readErr != nil {
					if readErr == io.EOF {
						log.Printf("[%s] Remote track %s (globalID: %s) ended (EOF). Cleaning up.", peerID, incomingTrack.ID(), globalTrackID)
					} else {
						log.Printf("[%s] Error reading from remote track %s (globalID: %s): %v", peerID, incomingTrack.ID(), globalTrackID, readErr)
					}
					s.removeTrack(globalTrackID) // Clean up the track from SFU
					return
				}

				// Write the received RTP packet to the local track (which is then sent to other peers)
				if _, writeErr := localTrack.Write(rtpBuf[:i]); writeErr != nil && writeErr != io.ErrClosedPipe {
					log.Printf("[%s] Error writing to local track %s (globalID: %s): %v", peerID, localTrack.ID(), globalTrackID, writeErr)
					return // Stop forwarding if write fails
				}
			}
		}()
	})

	// Set up ICE candidate handling: send candidates to the remote peer
	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			log.Printf("[%s] ICE Candidate gathering complete.", peerID)
			// Typically, the complete SDP (with all candidates) is sent after gathering is done,
			// or candidates are trickled. Here, we trickle.
			return
		}
		// log.Printf("[%s] Local ICE Candidate: %s", peerID, candidate.ToJSON().Candidate)

		candidateJSON, err := json.Marshal(candidate.ToJSON())
		if err != nil {
			log.Printf("[%s] Error marshalling local ICE candidate: %v", peerID, err)
			return
		}

		log.Println("sending ice candidate to client")
		// TODO: Send this candidate to your signaling server for peerID
		s.signalChannelSend <- &transport.SignalMessage{
			PeerID:    peerID,
			Type:      "candidate",
			Candidate: string(candidateJSON),
		}
	})

	// Handle PeerConnection state changes (e.g., disconnections)
	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf(" Peer Connection State has changed:", peerID, state.String())
		if state == webrtc.PeerConnectionStateFailed ||
			state == webrtc.PeerConnectionStateClosed ||
			state == webrtc.PeerConnectionStateDisconnected {
			s.cleanupPeer(peerID)
		}
	})

	peerConnection.OnICEConnectionStateChange(func(state webrtc.ICEConnectionState) {

		log.Printf(" Ice Connection State has changed:", peerID, state.String())
	})

	peerConnection.OnICEGatheringStateChange(func(state webrtc.ICEGathererState) {

		log.Printf(" Ice gather State has changed:", peerID, state.String())
	})

	// --- Add existing tracks from other peers to this new peer's connection ---
	// This must be done *before* CreateAnswer for these tracks to be included in the initial SDP answer.
	s.lock.RLock() // Read lock for iterating s.trackLocals
	log.Printf("[%s] Adding existing tracks from other peers to this new connection:", peerID)
	log.Println("local tracks found to send to other peers", s.trackLocals)
	for existingGlobalTrackID, localTrackToForward := range s.trackLocals {
		// Ensure the track doesn't originate from the current peer (it shouldn't be in trackLocals if it did, due to above logic)
		// A more robust check might involve parsing peerID from existingGlobalTrackID.
		log.Printf("[%s] Adding existing track %s (ID: %s, StreamID: %s, Kind: %s) to new peer connection",
			peerID, existingGlobalTrackID, localTrackToForward.ID(), localTrackToForward.StreamID(), localTrackToForward.Kind())
		if _, err := peerConnection.AddTrack(localTrackToForward); err != nil {
			log.Printf("[%s] Failed to add existing track %s to new peer: %v", peerID, existingGlobalTrackID, err)
		}
	}
	s.lock.RUnlock()

	// --- SDP Exchange ---
	log.Println("creating remote SDP")
	// Set the remote SessionDescription (the offer from the new peer)
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		log.Printf("[%s] Failed to set remote description: %v", peerID, err)
		s.cleanupPeer(peerID) // Clean up if setup fails
		return
	}

	test := peerConnection.SCTP().Transport().ICETransport().Role().String()
	fmt.Println(test, "this si it")

	// Create an SDP answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		log.Printf("[%s] Failed to create answer: %v", peerID, err)
		s.cleanupPeer(peerID)
		return
	}

	log.Println("creating local sdp")
	// Set the local SessionDescription (the SFU's answer)
	// This also starts the ICE gathering process if not already started.
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		log.Printf("[%s] Failed to set local description: %v", peerID, err)
		s.cleanupPeer(peerID)
		return
	}

	log.Printf("[%s] SDP Answer created and local description set. Sending to signaling server...", peerID)
	// TODO: Send this answer (answer.SDP) to your signaling server for peerID
	s.signalChannelSend <- &transport.SignalMessage{
		PeerID: peerID,
		Type:   "answer",
		SDP:    answer.SDP,
	}
}

// removeTrack cleans up a specific track from the SFU's internal state.
func (s *SFU) removeTrack(globalTrackID string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	trackToRemove, ok := s.trackLocals[globalTrackID]
	if !ok {
		return // Already removed or never existed
	}

	delete(s.trackLocals, globalTrackID)
	log.Printf("Removed track %s (globalID: %s) from SFU's trackLocals.", trackToRemove.ID(), globalTrackID)

	// Additionally, attempt to remove this track from all other peers' connections.
	// This is more complex as it requires finding the RTPSender.
	// For simplicity, often the client handles track ending, or the peer connection closing cleans this up.
	// If explicit removal is needed, you'd iterate s.peers, get their RTPSenders,
	// and call RemoveTrack if a sender is associated with trackToRemove.
	// Example (conceptual, needs RTPSender tracking):
	// for _, pcs := range s.peers {
	// for _, sender := range pcs.peerConnection.GetSenders() {
	// if sender.Track() != nil && sender.Track().ID() == trackToRemove.ID() && sender.Track().StreamID() == trackToRemove.StreamID() {
	// if err := pcs.peerConnection.RemoveTrack(sender); err != nil {
	// log.Printf("Error removing track %s from peer %s: %v", trackToRemove.ID(), pcs.id, err)
	// } else {
	// log.Printf("Removed track %s from sender of peer %s", trackToRemove.ID(), pcs.id)
	// }
	// }
	// }
	// }
}

// cleanupPeer removes a peer and its associated tracks from the SFU
func (s *SFU) cleanupPeer(peerID string) {
	s.lock.Lock() // Full lock for cleanup
	defer s.lock.Unlock()

	pcs, ok := s.peers[peerID]
	if !ok {
		log.Printf("Peer %s not found for cleanup or already cleaned up. Here is the peer list", s.peers)
		return
	}

	// Close the peer connection
	if pcs.peerConnection != nil {
		if err := pcs.peerConnection.Close(); err != nil {
			log.Printf("[%s] Error closing peer connection: %v", peerID, err)
		}
	}
	delete(s.peers, peerID)
	log.Printf("Peer %s and its PeerConnection removed.", peerID)

	// Remove tracks published by this peer from s.trackLocals
	// Iterate over trackLocals and identify tracks by peerID prefix in globalTrackID
	tracksToRemove := []string{}
	for globalTrackID := range s.trackLocals {
		// Assuming globalTrackID is "peerID_kind_trackID_ssrc"
		if len(globalTrackID) > len(peerID) && globalTrackID[:len(peerID)] == peerID && globalTrackID[len(peerID)] == '_' {
			tracksToRemove = append(tracksToRemove, globalTrackID)
		}
	}

	for _, trackIDToRemove := range tracksToRemove {
		log.Printf("[%s] Removing track %s (originated from this peer) from global trackLocals.", peerID, trackIDToRemove)
		delete(s.trackLocals, trackIDToRemove)
		// Note: These tracks are also implicitly removed from other peers when the RTP forwarding goroutine stops (due to EOF or error on the original track).
		// Explicitly calling RemoveTrack on other peers for these tracks is more robust but adds complexity.
	}
	log.Printf("Cleaned up tracks originated by peer %s.", peerID)
}
