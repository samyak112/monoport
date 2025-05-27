package multiplexer

import (
	"encoding/binary"
	"fmt"
	"github.com/pion/webrtc/v3"
	// "github.com/samyak112/monoport/sfu"
	"github.com/samyak112/monoport/stun_server"
	"log"
	"net"
)

// Detect WebRTC traffic (STUN, SFU)
func isSTUNPacket(data []byte) bool {
	return len(data) >= 20 &&
		(data[0] == 0x00 || data[0] == 0x01) &&
		binary.BigEndian.Uint32(data[4:8]) == 0x2112A442
}

func StartUdpMultiplexer(conn *net.UDPConn, webRtcApi *webrtc.API) {
	fmt.Println("listenint at 5000 for UDP")
	buf := make([]byte, 1500)
	for {
		//n -  buf may be larger than the packet, so only the first n bytes of it are valid data.
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		var udpResponse []byte
		if err != nil {
			log.Println("UDP read error:", err)
			continue
		}

		data := buf[:n]
		if isSTUNPacket(data) {
			udpResponse, err = stun_server.GetPacket(n, remoteAddr, buf)
		} else {
			// sfuResponse, err = sfu_server.SfuServer(conn)
			fmt.Println("reached the sfu") // Forward to SFU Server
		}

		_, err = conn.WriteToUDP(udpResponse, remoteAddr)
		if err != nil {
			fmt.Println("Error occured in writing UDP response", err)
		}

	}
}
