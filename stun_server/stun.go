package stun_server

import (
	"bytes"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/samyak112/monoport/transport"
	"log"
	"net"
)

func HandleStunPackets(conn *net.UDPConn, packetChannel chan transport.PacketInfo, iceUDPMux ice.UDPMux) {
	fmt.Println("listenint at 5000 for UDP")
	// buf := make([]byte, 1500)
	for pktInfo := range packetChannel {

		var udpResponse []byte
		// Access the actual packet data bytes
		dataPacket := pktInfo.Data

		// Access the source address
		remoteAddr := pktInfo.Addr
		err := pktInfo.Err

		if err != nil {
			log.Println("UDP read error:", err)
			continue
		}

		udpResponse, err = processStunPacket(pktInfo.N, pktInfo.Addr, dataPacket)
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

func processStunPacket(numBytes int, clientAddr *net.UDPAddr, buffer []byte) ([]byte, error) {
	msg := &stun.Message{
		Raw: make([]byte, numBytes),
	}

	copy(msg.Raw, buffer[:numBytes]) // Make a copy, as buf will be reused

	// fmt.Println(msg)
	var err = msg.Decode()
	if err != nil {
		fmt.Println("Error decoding STUN message:", err)
		return nil, fmt.Errorf("Error occured in decoding the message", err)
	}

	if msg.Type == stun.MessageType(stun.BindingRequest) {

		response, err := stun.Build(
			stun.BindingSuccess,
			stun.NewTransactionIDSetter(msg.TransactionID),
			&stun.XORMappedAddress{
				IP:   clientAddr.IP,
				Port: clientAddr.Port,
			})

		if err != nil {
			return nil, fmt.Errorf("error building STUN response: %w", err)
		}

		// log.Println(msg, "this is the packet")

		var buf bytes.Buffer
		if _, err := response.WriteTo(&buf); err != nil {
			return nil, fmt.Errorf("error serializing STUN response: %w", err)
		}

		// fmt.Println("Sending STUN BindingSuccess to", clientAddr)
		return buf.Bytes(), nil

	} else {
		// fmt.Println("Recieved non Binding", msg)
		return nil, fmt.Errorf("Received non-BindingRequest STUN message typ", msg.Type)
	}
}
