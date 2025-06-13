package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/samyak112/monoport/transport"
	"log"
	"net"
	"strings"
)

// ICECandidate represents the structure of a WebRTC ICE candidate for JSON serialization.
// This structure is based on the common format used in signaling.
type ICECandidate struct {
	Foundation     string `json:"foundation"`
	Priority       uint32 `json:"priority"`
	Address        string `json:"address"`
	Protocol       string `json:"protocol"`
	Port           uint16 `json:"port"`
	Typ            string `json:"type"` // "host", "srflx", "prflx", "relay"
	Component      uint16 `json:"component"`
	RelatedAddress string `json:"relatedAddress,omitempty"`
	RelatedPort    uint16 `json:"relatedPort,omitempty"`
	TCPType        string `json:"tcpType,omitempty"`
}

// FinalCandidatePayload is the structure that will be marshalled to JSON and sent
// to the client. It matches the format expected by new RTCIceCandidate().
type FinalCandidatePayload struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex uint16 `json:"sdpMLineIndex"`
}

func (c *ICECandidate) ToSDP() string {
	// Example: "candidate:1 1 udp 2122252543 192.168.1.10 8421 typ srflx"
	return fmt.Sprintf("candidate:%s %d %s %d %s %d typ %s",
		c.Foundation,
		c.Component,
		c.Protocol,
		c.Priority,
		c.Address,
		c.Port,
		c.Typ,
	)
}

// newServerReflexiveCandidate creates an ICECandidate struct representing a
// server reflexive (srflx) candidate.
func newServerReflexiveCandidate(clientAddr *net.UDPAddr) (*ICECandidate, error) {
	// A foundation is used to group related candidates. For a simple srflx
	// candidate from a STUN server, we can generate a simple one.
	foundation := "1"

	// Priority for a server-reflexive candidate. This is a typical value.
	// Priority = (2^24)*type_preference + (2^8)*local_preference + (256 - component_id)
	priority := (1<<24)*100 + (1<<8)*65535 + (256 - 1)

	candidate := &ICECandidate{
		Foundation: foundation,
		Priority:   uint32(priority),
		Address:    clientAddr.IP.String(),
		Protocol:   "udp",
		Port:       uint16(clientAddr.Port),
		Typ:        "srflx", // Server Reflexive type
		Component:  1,       // Typically 1 for RTP
	}

	return candidate, nil
}

func HandleStunPackets(conn *net.UDPConn, packetChannel chan transport.PacketInfo, iceUDPMux ice.UDPMux, signalingInstance *Signal) {
	fmt.Println("listenint at 5000 for UDP")
	for pktInfo := range packetChannel {

		var udpResponse []byte
		var ufrag string
		var msgType string
		// Access the actual packet data bytes
		dataPacket := pktInfo.Data

		// Access the source address
		remoteAddr := pktInfo.Addr
		err := pktInfo.Err

		if err != nil {
			log.Println("UDP read error:", err)
			continue
		}

		udpResponse, ufrag, msgType, err = processStunPacket(pktInfo.N, pktInfo.Addr, dataPacket)
		// fmt.Println(udpResponse)
		if err != nil {
			fmt.Println("not sending the stun response", err)
		} else {

			// _, err = conn.WriteToUDP(udpResponse, remoteAddr)
			if msgType == "messageIntegrity" {
				payload := map[string]interface{}{
					"type":          "stun-candidate",
					"stunCandidate": string(udpResponse),
				}

				data, payloadErr := json.Marshal(payload)
				if payloadErr != nil {
					log.Println("JSON marshal error in stun:", err)
					break
				}

				if err2 := signalingInstance.UfragMap[ufrag].WriteMessage(1, data); err2 != nil {
					fmt.Println("something went wrong")
				}
			} else {
				if err != nil {
					fmt.Println("not sending the stun response")
				} else {

					_, err = conn.WriteToUDP(udpResponse, remoteAddr)
					if err != nil {
						fmt.Println("Error occured in writing UDP response", err)
					}
				}
			}
		}

	}
}

// processStunPacket inspects a raw STUN packet and returns the appropriate response.
// If it's a simple STUN request, it returns a binary STUN response.
// If it's an ICE connectivity check, it returns a JSON ICE candidate.
func processStunPacket(numBytes int, clientAddr *net.UDPAddr, buffer []byte) ([]byte, string, string, error) {
	msg := &stun.Message{
		Raw: make([]byte, numBytes),
	}
	copy(msg.Raw, buffer[:numBytes])

	// Decode the raw bytes into a STUN message.
	if err := msg.Decode(); err != nil {
		return nil, "", "", fmt.Errorf("error decoding STUN message: %w", err)
	}

	// We only handle Binding Requests.
	if msg.Type != stun.BindingRequest {
		return nil, "", "", fmt.Errorf("received non-BindingRequest STUN message type: %s", msg.Type)
	}

	// The request is a BindingRequest. Now check if it's for ICE or traditional STUN.
	// The presence of MessageIntegrity indicates an ICE connectivity check.
	if msg.Contains(stun.AttrMessageIntegrity) {
		// --- This is an ICE connectivity check ---
		// The client expects a JSON ICE candidate, not a binary STUN response.

		ufrag, err := getRemoteUfragFromMessage(msg)
		if err != nil {
			return nil, "", "", err
		}

		// 1. Create the server-reflexive ICE candidate struct from the client's public address.
		iceCandidate, err := newServerReflexiveCandidate(clientAddr)
		if err != nil {
			return nil, "", "", fmt.Errorf("error creating ICE candidate: %w", err)
		}

		// 2. Create the final payload for the client.
		// Note: The server doesn't know the sdpMid or mLineIndex. We use default
		// values. The client may need to adjust these if it has multiple media sections.
		// For most simple cases ("audio" or "video" only), this works directly.
		finalPayload := FinalCandidatePayload{
			Candidate:     iceCandidate.ToSDP(),
			SdpMid:        "0", // Defaulting to "0" (first media line)
			SdpMLineIndex: 0,   // Defaulting to the first media line
		}

		// 3. Marshal the final payload into JSON. This is what you'll send over WebSocket.
		jsonPayload, err := json.Marshal(finalPayload)
		if err != nil {
			return nil, "", "", fmt.Errorf("error marshaling final payload to JSON: %w", err)
		}

		// 3. Return the JSON payload, the ufrag, and the message type.
		return jsonPayload, ufrag, "messageIntegrity", nil

	} else {
		// --- This is a traditional STUN request ---
		// The client expects a binary STUN BindingSuccess response.
		fmt.Println("-> Handling traditional STUN request (no MessageIntegrity)...")

		response, err := stun.Build(
			stun.BindingSuccess,
			stun.NewTransactionIDSetter(msg.TransactionID),
			&stun.XORMappedAddress{
				IP:   clientAddr.IP,
				Port: clientAddr.Port,
			},
		)
		if err != nil {
			return nil, "", "", fmt.Errorf("error building STUN response: %w", err)
		}

		var buf bytes.Buffer
		if _, err := response.WriteTo(&buf); err != nil {
			return nil, "", "", fmt.Errorf("error serializing STUN response: %w", err)
		}

		// Return the binary response, no ufrag, and "normal" type.
		return buf.Bytes(), "", "normal", nil
	}
}

func getRemoteUfragFromMessage(msg *stun.Message) (string, error) {
	if msg.Type != stun.BindingRequest {
		return "", fmt.Errorf("not a BindingRequest")
	}

	var username stun.Username
	if err := username.GetFrom(msg); err != nil {
		return "", fmt.Errorf("failed to get USERNAME attribute: %w", err)
	}

	parts := strings.SplitN(string(username), ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed USERNAME attribute: %s", username)
	}

	remoteUfrag := parts[0]
	return remoteUfrag, nil
}
