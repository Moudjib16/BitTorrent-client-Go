package p2p

import (
	"bytes"
	"fmt"
	"io"
)

const protocolName = "BitTorrent protocol"

// The handshake is always 68 bytes: 1 length byte, 19 protocol bytes,
// 8 reserved bytes, 20 info hash bytes and 20 peer id bytes.
const handshakeLen = 68

func handshake(rw io.ReadWriter, infoHash, peerID [20]byte) error {
	req := make([]byte, handshakeLen)
	req[0] = byte(len(protocolName))
	copy(req[1:20], protocolName)
	copy(req[28:48], infoHash[:])
	copy(req[48:68], peerID[:])

	if _, err := rw.Write(req); err != nil {
		return fmt.Errorf("send handshake: %w", err)
	}

	resp := make([]byte, handshakeLen)
	if _, err := io.ReadFull(rw, resp); err != nil {
		return fmt.Errorf("read handshake: %w", err)
	}
	if resp[0] != byte(len(protocolName)) || string(resp[1:20]) != protocolName {
		return fmt.Errorf("peer does not speak the BitTorrent protocol")
	}
	if !bytes.Equal(resp[28:48], infoHash[:]) {
		return fmt.Errorf("peer returned a different info hash")
	}
	return nil
}
