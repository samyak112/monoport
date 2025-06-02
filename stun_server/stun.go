package stun_server

import (
	"encoding/binary"
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/samyak112/monoport/transport"
	"log"
	"net"
)

// Detect WebRTC traffic (STUN, SFU)
func isSTUNPacket(data []byte) bool {
	return len(data) >= 20 &&
		(data[0] == 0x00 || data[0] == 0x01) &&
		binary.BigEndian.Uint32(data[4:8]) == 0x2112A442
}

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

		if isSTUNPacket(dataPacket) {
			udpResponse, err = processStunPacket(pktInfo.N, pktInfo.Addr, dataPacket)
		} else {
			// not a stun packet so we dont need to handle it
		}

		_, err = conn.WriteToUDP(udpResponse, remoteAddr)
		if err != nil {
			fmt.Println("Error occured in writing UDP response", err)
		}

	}
}

func processStunPacket(numBytes int, clientAddr *net.UDPAddr, buffer []byte) ([]byte, error) {
	msg := &stun.Message{
		Raw: make([]byte, numBytes),
	}

	copy(msg.Raw, buffer[:numBytes]) // Make a copy, as buf will be reused

	fmt.Println(msg)
	var err = msg.Decode()
	if err != nil {
		fmt.Println("Error decoding STUN message:", err)
		return nil, fmt.Errorf("Error occured in decoding the message", err)
	}

	fmt.Println(msg.Type)
	if msg.Type == stun.MessageType(stun.BindingRequest) {

		response, err := stun.Build(
			stun.BindingSuccess,
			stun.NewTransactionIDSetter(msg.TransactionID),
			&stun.XORMappedAddress{
				IP:   clientAddr.IP,
				Port: clientAddr.Port,
			})

		if err != nil {
			fmt.Println("this is the error", err)
			return nil, fmt.Errorf("Error building STUN response", err)
		}

		fmt.Println("sending the packet")
		return response.Raw, nil

	} else {
		fmt.Println("Recieved non Binding", msg.Type)
		return nil, fmt.Errorf("Received non-BindingRequest STUN message typ", msg.Type)
	}
}
