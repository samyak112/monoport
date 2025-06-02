package ws

func (s *Signal) AddPeer(peerID string, peer *SignalingPeer) {
	s.signalLock.Lock()
	defer s.signalLock.Unlock()

	if s.PeerMap == nil {
		s.PeerMap = make(map[string]*SignalingPeer)
	}

	s.PeerMap[peerID] = peer
}
