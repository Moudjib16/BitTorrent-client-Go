package p2p

import (
	"net"
	"strconv"
)

// Peer is another client in the swarm.
type Peer struct {
	IP   net.IP
	Port uint16
}

func (p Peer) String() string {
	return net.JoinHostPort(p.IP.String(), strconv.Itoa(int(p.Port)))
}
