package sfu_server

import (
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/webrtc/v3"
	"github.com/samyak112/monoport/logger"
	"net"
)

/*
CreateCustomUDPWebRTCAPI configures and returns a WebRTC API instance that utilizes a
pre-existing UDP connection instead of opening new ports.

This is achieved by using a webrtc.SettingEngine to configure a
webrtc.NewICEUDPMux with the provided UDP connection (conn).
Using an ICEUDPMux allows multiple PeerConnections to share a single
UDP port, which helps in conserving ports. The returned webrtc.API applies
these settings to all subsequently created PeerConnections.

Args:

	conn (net.PacketConn): The existing UDP connection to be used by WebRTC.

Returns:

	*webrtc.API: A configured WebRTC API for creating PeerConnections.
*/
func CreateCustomUDPWebRTCAPI(conn net.PacketConn) (*webrtc.API, ice.UDPMux) {

	/*SettingEngine must be configured with the UDP multiplexer before creating
	PeerConnections because ICE transport configuration is immutable after
	initialization. The ICE agent needs to know about the shared socket during
	setup to properly register for packet demultiplexing and generate
	appropriate candidates referencing the multiplexed port.

	However, it doesn't actually do anything by itself - it just stores your preferences we have to call
	a newAPI function bring this in effect.*/
	settingEngine := webrtc.SettingEngine{}

	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
	})

	/*Tow reasons to use pion's udp multiplexer here
	1.NewICEUDPMux creates a UDP multiplexer for efficient port sharing across
	multiple PeerConnections. Prevents port exhaustion and
	reducing NAT mappings and system resource usage.

	2. Another reason and more important reason is I need a way to pass in my own UDP conn which i created
	in main.go so that i can bypass symmetric NATs*/
	logger := &logger.SimpleLogger{}
	var udpMux = webrtc.NewICEUDPMux(logger, conn)
	settingEngine.SetICEUDPMux(udpMux)

	m := &webrtc.MediaEngine{}

	// 2. Register the default codecs that browsers support.
	// THIS IS THE CRUCIAL STEP. Without it, the SFU doesn't know how
	// to handle video (VP8, H264) or audio (Opus) from a browser.
	if err := m.RegisterDefaultCodecs(); err != nil {
		// This is a fatal startup error, so panic is appropriate.
		fmt.Println("Failed to register default codecs: %v", err)
	}

	/*NewAPI creates a configured WebRTC API factory from SettingEngine options.
	Returns an API instance that applies custom settings to all created
	PeerConnections. Initialize once per application, not per connection.*/
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine), webrtc.WithMediaEngine(m))

	return api, udpMux
}

func RecvAndForwardMediaPackets(webRtcApi *webrtc.API) {

	fmt.Println("reached inside this")
	_, err := webRtcApi.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		panic(err)
	}

}
