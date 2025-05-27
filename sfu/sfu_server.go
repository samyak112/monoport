package sfu_server

import (
	"fmt"
	"github.com/pion/webrtc/v3"
	"net"
)

// SfuServer configures and returns a WebRTC API instance that utilizes a
// pre-existing UDP connection instead of opening new ports.
//
// This is achieved by using a webrtc.SettingEngine to configure a
// webrtc.NewICEUDPMux with the provided UDP connection (conn).
// Using an ICEUDPMux allows multiple PeerConnections to share a single
// UDP port, which helps in conserving ports. The returned webrtc.API applies
// these settings to all subsequently created PeerConnections.
//
// Args:
//
//	conn (*net.UDPConn): The existing UDP connection to be used by WebRTC.
//
// Returns:
//
//	*webrtc.API: A configured WebRTC API for creating PeerConnections.
func CreateCustomUDPWebRTCAPI(conn *net.UDPConn) *webrtc.API {
	fmt.Println("is it going inside again and again?")

	// SettingEngine must be configured with the UDP multiplexer before creating
	// PeerConnections because ICE transport configuration is immutable after
	// initialization. The ICE agent needs to know about the shared socket during
	// setup to properly register for packet demultiplexing and generate
	// appropriate candidates referencing the multiplexed port.

	// However, it doesn't actually do anything by itself - it just stores your preferences we have to call
	// a newAPI function bring this in effect.
	settingEngine := webrtc.SettingEngine{}

	settingEngine.SetNetworkTypes([]webrtc.NetworkType{
		webrtc.NetworkTypeUDP4,
	})

	// Tow reasons to use pion's udp multiplexer here
	// 1.NewICEUDPMux creates a UDP multiplexer for efficient port sharing across
	// multiple PeerConnections. Prevents port exhaustion and
	// reducing NAT mappings and system resource usage.

	// 2. Another reason and more important reason is I need a way to pass in my own UDP conn which i created
	// in main.go so that i can bypass symmetric NATs
	var udpMux = webrtc.NewICEUDPMux(nil, conn)
	settingEngine.SetICEUDPMux(udpMux)

	// NewAPI creates a configured WebRTC API factory from SettingEngine options.
	// Returns an API instance that applies custom settings to all created
	// PeerConnections. Initialize once per application, not per connection.
	api := webrtc.NewAPI(webrtc.WithSettingEngine(settingEngine))

	return api
}
