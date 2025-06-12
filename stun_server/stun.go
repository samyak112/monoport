package stun_server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/samyak112/monoport/signaling"
	"github.com/samyak112/monoport/transport"
	"log"
	"net"
	"strings"
)

func HandleStunPackets(conn *net.UDPConn, packetChannel chan transport.PacketInfo, iceUDPMux ice.UDPMux, signalingInstance *ws.Signal) {
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
					"stunCandidate": udpResponse,
				}

				data, payloadErr := json.Marshal(payload)
				if payloadErr != nil {
					log.Println("JSON marshal error in stun:", err)
					break
				}
				fmt.Println("derefencing below")
				fmt.Println(*signalingInstance.UfragMap[ufrag])
				signalingInstance.SendCandidate(ufrag, data)
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

func processStunPacket(numBytes int, clientAddr *net.UDPAddr, buffer []byte) ([]byte, string, string, error) {
	msg := &stun.Message{
		Raw: make([]byte, numBytes),
	}

	copy(msg.Raw, buffer[:numBytes]) // Make a copy, as buf will be reused

	// fmt.Println(msg)
	var err = msg.Decode()
	if err != nil {
		fmt.Println("Error decoding STUN message:", err)
		return nil, "", "", fmt.Errorf("Error occured in decoding the message", err)
	}

	// Note : Not handling BindingRequests which contains Message Integrity because
	// thats not the job of a STUN server, this server is solely here to help a client
	// know its public address and port
	fmt.Println("newRequest", msg)

	// what am trying to achieve here is that as soon as my frontend sent a connectivity check
	// I will get its public IP and port and create an ICE candidate and will send it to frontend
	// now frontend got a new ice candidate and it can use it for a new connectivity checkk
	if msg.Type == stun.MessageType(stun.BindingRequest) {

		var ufrag string
		response, err := stun.Build(
			stun.BindingSuccess,
			stun.NewTransactionIDSetter(msg.TransactionID),
			&stun.XORMappedAddress{
				IP:   clientAddr.IP,
				Port: clientAddr.Port,
			})

		msgType := "normal"

		if msg.Contains(stun.AttrMessageIntegrity) {

			ufrag, err2 := getRemoteUfragFromMessage(msg)

			fmt.Println("was there any error in ufrag?", err2, ufrag)

			if err != nil {
				return nil, "", "", fmt.Errorf("error building STUN response: %w", err)
			} else {
				msgType = "messageIntegrity"
			}
		}

		var buf bytes.Buffer
		if _, err := response.WriteTo(&buf); err != nil {
			return nil, "", "", fmt.Errorf("error serializing STUN response: %w", err)
		}

		fmt.Println("got stun message", ufrag)
		return buf.Bytes(), ufrag, msgType, nil

	} else {
		// fmt.Println("Recieved non Binding", msg)
		return nil, "", "", fmt.Errorf("Received non-BindingRequest STUN message typ", msg.Type)
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
