// Package transport provides types and interfaces for custom UDP packet handling,
// including wrappers for net.PacketConn and structured packet forwarding.

package transport

import (
	"encoding/binary"
	"fmt"
	"net"
)

type PacketInfo struct {
	Data []byte       // The actual packet data
	Addr *net.UDPAddr // Where it came from
	Err  error        // Any error during read
	N    int
}
type CustomPacketConn struct {
	*net.UDPConn
	DataForwardChan chan PacketInfo // Channel to send data out
}

// Detect WebRTC traffic (STUN, SFU)
func (c *CustomPacketConn) isSTUNPacket(data []byte) bool {
	return len(data) >= 20 &&
		(data[0] == 0x00 || data[0] == 0x01) &&
		binary.BigEndian.Uint32(data[4:8]) == 0x2112A442
}

func (c *CustomPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {

	var udpAddr *net.UDPAddr
	n, udpAddr, err = c.UDPConn.ReadFromUDP(p)

	/* Not checking if a packet is for STUN or for RTP because RTP packets should not be lagged
	   even by ms just because of checking of STUN packet i can transfer this packet and check*/

	// If data was read (or even if there was an error, send info)
	if c.DataForwardChan != nil {
		isStunPacket := c.isSTUNPacket(p)

		if isStunPacket {
			// IMPORTANT: Make a copy of the data for the channel.
			// Note: 'p' buffer is reused internally by Pion, so a deep copy is mandatory before sending
			dataCopy := make([]byte, n)
			copy(dataCopy, p[:n])

			select {
			// Data info sent successfully
			case c.DataForwardChan <- PacketInfo{Data: dataCopy, Addr: udpAddr, Err: err, N: n}:
			default:
				fmt.Println("Packet dropped because of full channel")
				// Channel is full, so data is dropped.
				// This prevents ReadFrom from blocking Pion.
			}
		}
	}

	fmt.Println("packet reached pion")
	return n, udpAddr, err
}

func (c *CustomPacketConn) Close() error {

	if c.DataForwardChan != nil {
		close(c.DataForwardChan)
	}
	return c.UDPConn.Close()
}

// SignalMessage is a generic struct for messages to/from signaling server
type SignalMessage struct {
	PeerID    string `json:"peerId,omitempty"`
	Type      string `json:"type"` // "offer", "answer", "candidate"
	SDP       string `json:"sdp,omitempty"`
	Candidate string `json:"candidate,omitempty"` // JSON string of webrtc.ICECandidateInit
}
