package multiplexer

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

// Detect WebRTC traffic (STUN, SFU)
func isSTUNPacket(data []byte) bool {
	return len(data) >= 20 &&
		(data[0] == 0x00 || data[0] == 0x01) &&
		binary.BigEndian.Uint32(data[4:8]) == 0x2112A442
}

func StartUdpMultiplexer(conn *net.UDPConn) {
	fmt.Println("UDP listening on 5000")
	buf := make([]byte, 1500)
	for {
		// not using addr parameter here because i guess stun lib and sfu lib will handle that
		// but I'll add it back here incase its needed and i need to handle the reply manually
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("UDP read error:", err)
			continue
		}

		data := buf[:n]
		if isSTUNPacket(data) {
			fmt.Println("Reached to stun") // Forward to STUN Server
		} else {
			fmt.Println("reached the sfu") // Forward to SFU Server
		}

		// _ = addr // optionally use addr
	}
}
