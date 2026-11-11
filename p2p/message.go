package p2p

import (
	"encoding/binary"
	"fmt"
	"io"
)

// The message IDs from the BitTorrent protocol specification.
type messageID byte

const (
	msgChoke messageID = iota
	msgUnchoke
	msgInterested
	msgNotInterested
	msgHave
	msgBitfield
	msgRequest
	msgPiece
	msgCancel
)

// 16 MiB is well above the largest piece size we should ever be offered, but
// small enough that a broken peer cannot make us allocate unlimited memory.
const maxMessageSize = 16 << 20

// Message is one peer wire message. A nil message is a keep-alive.
type Message struct {
	ID      messageID
	Payload []byte
}

func readMessage(r io.Reader) (*Message, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil // keep-alive
	}
	if length > maxMessageSize {
		return nil, fmt.Errorf("peer sent an oversized message (%d bytes)", length)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return &Message{ID: messageID(buf[0]), Payload: buf[1:]}, nil
}

func writeMessage(w io.Writer, id messageID, payload []byte) error {
	buf := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)+1))
	buf[4] = byte(id)
	copy(buf[5:], payload)
	_, err := w.Write(buf)
	return err
}
