package ws

import (
	"github.com/gorilla/websocket"
	"github.com/samyak112/monoport/transport"
	"sync"
)

type Signal struct {
	PeerMap           map[string]*websocket.Conn
	UfragMap          map[string]*websocket.Conn
	SignalLock        sync.Mutex
	SignalChannelRecv chan *transport.SignalMessage
}
