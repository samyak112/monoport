package main

import (
	"github.com/gorilla/websocket"
	"github.com/samyak112/monoport/sfu"
	"github.com/samyak112/monoport/signaling"
	"github.com/samyak112/monoport/stun_server"
	"github.com/samyak112/monoport/transport"
	"log"
	"net"
	"net/http"
)

func main() {
	/* These are pointers so we can keep all peers in one shared map.
	If they weren’t pointers, we’d end up with copies, and each one would have its own mutex,
	which means locking wouldn’t work properly — they’d all be locking different instances. */

	packetChannel := make(chan transport.PacketInfo, 1024)
	signalingChannel := make(chan *transport.SignalMessage, 5)

	// returns a *net.UDPAddr struct representing the UDP network address, using the network type and address
	udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")

	// using udpAddr to bind the UDP socket or send packets to the given address.
	udpConn, _ := net.ListenUDP("udp", udpAddr)

	myConn := &transport.CustomPacketConn{UDPConn: udpConn, DataForwardChan: packetChannel}
	webRtcApi, iceUDPMux := sfu_server.CreateCustomUDPWebRTCAPI(myConn)

	//passing same signalingChannel in both sfu and signaling struct creation
	// so that i can send something to the channel from one struct and it can be
	// received in another , this way i can transfer information from sfu to signaling
	// by keeping their logic different

	sfu := sfu_server.NewSFU(webRtcApi, signalingChannel)

	signaling := &ws.Signal{
		PeerMap:           make(map[string]*websocket.Conn),
		SignalChannelRecv: signalingChannel,
	}
	go signaling.ProcessOutgoingSignals()

	// using a go routine so that the TCP connection is not blocked because of the UDP stream
	go stun_server.HandleStunPackets(udpConn, packetChannel, iceUDPMux)

	//Start WebSocket signaling
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		ws.HandleSDP(w, r, sfu, signaling)
	})
	log.Println("Listening on :8000")
	http.ListenAndServe("127.0.0.1:8000", nil)

}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
