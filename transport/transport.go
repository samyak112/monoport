// Package transport provides types and interfaces for custom UDP packet handling,
// including wrappers for net.PacketConn and structured packet forwarding.

package transport

import (
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

func (c *CustomPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {

	var udpAddr *net.UDPAddr
	n, udpAddr, err = c.UDPConn.ReadFromUDP(p)

	// If data was read (or even if there was an error, send info)
	if c.DataForwardChan != nil {
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

	fmt.Println("packet reached pion")
	return n, udpAddr, err
}

func (c *CustomPacketConn) Close() error {

	if c.DataForwardChan != nil {
		close(c.DataForwardChan)
	}
	return c.UDPConn.Close()
}
