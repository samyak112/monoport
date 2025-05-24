package main

import (
	"log"
	"net"
	"net/http"

	// "main/sfu"
	ws "github.com/samyak112/monoport/signaling"
	// "main/stun"
	multiplexer "github.com/samyak112/monoport/udp"
)

func main() {
	// returns a *net.UDPAddr struct representing the UDP network address, using the network type and address
	udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")

	// using udpAddr to bind the UDP socket or send packets to the given address.
	udpConn, _ := net.ListenUDP("udp", udpAddr)

	// using a go routine so that the TCP connection is not blocked because of the UDP stream
	go multiplexer.StartUdpMultiplexer(udpConn)

	//Start WebSocket signaling
	http.HandleFunc("/ws", ws.WebsocketHandler)
	log.Println("Listening on :8000")
	http.ListenAndServe(":8000", nil)
}
