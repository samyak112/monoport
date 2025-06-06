package ws

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/sfu"
	"github.com/samyak112/monoport/transport"
	"log"
	"net/http"
)

// Handles incoming WebSocket signaling
func HandleSDP(w http.ResponseWriter, r *http.Request, sfuInstance *sfu_server.SFU, signalingInstance *Signal) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		_, rawMessage, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		var msg transport.SignalMessage
		if err := json.Unmarshal(rawMessage, &msg); err != nil {
			log.Printf("Error unmarshalling signaling message: %v. Message: %s", err, rawMessage)
			return
		}
		switch msg.Type {
		case "offer":
			if msg.PeerID == "" || msg.SDP == "" {
				log.Println("Invalid offer message: missing peerId or sdp")
				return
			}
			offer := webrtc.SessionDescription{
				Type: webrtc.SDPTypeOffer,
				SDP:  msg.SDP,
			}
			go sfuInstance.HandleNewPeerOffer(msg.PeerID, offer)

		case "ice-candidate":
			if msg.PeerID == "" || msg.Candidate == "" {
				log.Println("Invalid candidate message: missing peerId or candidate")
				return
			}
			go sfuInstance.HandleIceCandidate(msg.PeerID, msg.Candidate)

		case "join-room":
			go signalingInstance.AddPeer(msg.PeerID, conn)

		default:
			log.Printf("Unhandled signaling message type: %s", msg.Type)
		}
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
