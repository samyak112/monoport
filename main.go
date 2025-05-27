package main

import (
	"log"
	"net"
	"net/http"

	// "main/sfu"
	"github.com/samyak112/monoport/sfu"
	ws "github.com/samyak112/monoport/signaling"
	// "main/stun"
	multiplexer "github.com/samyak112/monoport/udp"
)

func main() {

	// for now am keeping a map in memory but this cant be used if we have multiple instances of our server
	// in that case we will have to use redis or kafka or some other service to manage the rooms and users at a shared space
	// var rooms = map[string][]string{} // roomID -> list of connection IDs

	// returns a *net.UDPAddr struct representing the UDP network address, using the network type and address
	udpAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:5000")

	// using udpAddr to bind the UDP socket or send packets to the given address.
	udpConn, _ := net.ListenUDP("udp", udpAddr)

	webRtcApi := sfu_server.CreateCustomUDPWebRTCAPI(udpConn)

	// using a go routine so that the TCP connection is not blocked because of the UDP stream
	go multiplexer.StartUdpMultiplexer(udpConn, webRtcApi)

	//Start WebSocket signaling
	http.HandleFunc("/", ws.WebsocketHandler)
	log.Println("Listening on :8000")
	http.ListenAndServe("127.0.0.1:8000", nil)

}
