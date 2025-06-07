package sfu_server

import (
	"fmt"
	"github.com/pion/ice/v2"
	"github.com/pion/webrtc/v3"
	"log"
	"net"
)

// SimpleLogger implements LeveledLogger by printing to standard log
type SimpleLogger struct{}

// helper to print message with prefix + static message appended
func (l *SimpleLogger) log(level, msg string) {
	staticMsg := " [this is from custom logger]"
	log.Printf("[%s] %s%s", level, msg, staticMsg)
}

// helper to print formatted message with prefix + static message appended
func (l *SimpleLogger) logf(level, format string, args ...interface{}) {
	// Format the original message
	msg := fmt.Sprintf(format, args...)
	// Append static message
	staticMsg := " [this is from custom logger]"
	log.Printf("[%s] %s%s", level, msg, staticMsg)
}

func (l *SimpleLogger) Trace(msg string) { l.log("TRACE", msg) }
func (l *SimpleLogger) Tracef(format string, args ...interface{}) {
	l.logf("TRACE", format, args...)
}
func (l *SimpleLogger) Debug(msg string) { l.log("DEBUG", msg) }
func (l *SimpleLogger) Debugf(format string, args ...interface{}) {
	l.logf("DEBUG", format, args...)
}
func (l *SimpleLogger) Info(msg string) { l.log("INFO", msg) }
func (l *SimpleLogger) Infof(format string, args ...interface{}) {
	l.logf("INFO", format, args...)
}
func (l *SimpleLogger) Warn(msg string) { l.log("WARN", msg) }
func (l *SimpleLogger) Warnf(format string, args ...interface{}) {
	l.logf("WARN", format, args...)
}
func (l *SimpleLogger) Error(msg string) { l.log("ERROR", msg) }
func (l *SimpleLogger) Errorf(format string, args ...interface{}) {
	l.logf("ERROR", format, args...)
}

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
	logger := &SimpleLogger{}
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
