package multiplexer

import (
	"encoding/binary"
	"fmt"
	// "github.com/pion/webrtc/v3"
	// "github.com/samyak112/monoport/sfu"
	"github.com/pion/ice/v2"
	"github.com/samyak112/monoport/stun_server"
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

func StartUdpMultiplexer(conn *net.UDPConn, packetChannel chan transport.PacketInfo, iceUDPMux ice.UDPMux) {
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
			udpResponse, err = stun_server.GetPacket(pktInfo.N, pktInfo.Addr, dataPacket)
		} else {
			// sfuResponse, err = sfu_server.RecvAndForwardMediaPackets(webRtcApi, sdpChannel)

			fmt.Println("reached the sfu") // Forward to SFU Server:w
			// iceUDPMux.HandleUDPPacket(r remoteAddr, buf[:n])
			// sfu_server.RecvAndForwardMediaPackets(webRtcApi)
		}

		_, err = conn.WriteToUDP(udpResponse, remoteAddr)
		if err != nil {
			fmt.Println("Error occured in writing UDP response", err)
		}

	}
}
