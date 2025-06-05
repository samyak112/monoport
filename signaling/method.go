package ws

import (
	"github.com/gorilla/websocket"
	"log"
)

func (s *Signal) AddPeer(peerID string, conn *websocket.Conn) {
	s.SignalLock.Lock()
	defer s.SignalLock.Unlock()

	if s.PeerMap == nil {
		s.PeerMap = make(map[string]*websocket.Conn)
	}

	s.PeerMap[peerID] = conn
	log.Println(s.PeerMap)
	log.Println("peer added", peerID)
	peer, ok := s.PeerMap[peerID]

	if ok {
		log.Printf("Peer: %+v\n", *peer) // <- dereferencing here to print the actual struct
	} else {
		log.Println("Peer not found")
	}
}

func (s *Signal) RemovePeer(peerID string) {
	s.SignalLock.Lock()
	defer s.SignalLock.Unlock()

	if s.PeerMap != nil {
		delete(s.PeerMap, peerID)
	}
}

// processOutgoingSignals simulates sending messages from the SFU to clients via a signaling server.
func (s *Signal) ProcessOutgoingSignals() {
	log.Println("atleast got in")
	for msg := range s.SignalChannelRecv {
		// TODO: Implement actual sending logic to your signaling server.
		// This would involve formatting the message (e.g., JSON) and sending it
		// over WebSocket, HTTP, or another transport to the specific peer (msg.PeerID).
		log.Printf(">>> OUTGOING SIGNAL for Peer %s (Type: %s) >>>", msg.PeerID, msg.Type)
		log.Println(s.PeerMap)
		if msg.SDP != "" {
			log.Printf("SDP for %s:\n%s", msg.PeerID, msg.SDP)
		}
		if msg.Candidate != "" {
			// The candidate string is already a JSON of ICECandidateInit
			log.Printf("ICE Candidate for %s: %s", msg.PeerID, msg.Candidate)
		}
		log.Println(">>> END OUTGOING SIGNAL >>>")
	}
}
