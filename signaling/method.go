package ws

import (
	"encoding/json"
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
	log.Println(peerID)

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
	for msg := range s.SignalChannelRecv {
		if msg.SDP != "" {
			// log.Printf("SDP for %s:\n%s", msg.PeerID, msg.SDP)

			payload := map[string]interface{}{
				"type": msg.Type,
				"sdp":  msg.SDP,
			}

			data, err := json.Marshal(payload)
			if err != nil {
				log.Println("JSON marshal error:", err)
				break
			}

			if err := s.PeerMap[msg.PeerID].WriteMessage(1, data); err != nil {
				log.Println("Write error in sending SDP:", err)
				break
			}
		}
		if msg.Candidate != "" {
			// The candidate string is already a JSON of ICECandidateInit
			payload := map[string]interface{}{
				"type":      msg.Type,
				"candidate": msg.Candidate,
			}

			data, err := json.Marshal(payload)
			if err != nil {
				log.Println("JSON marshal error:", err)
				break
			}
			if err := s.PeerMap[msg.PeerID].WriteMessage(1, data); err != nil {
				log.Println("Write error in Candidate:", err)
				break
			}
		}
	}
}
