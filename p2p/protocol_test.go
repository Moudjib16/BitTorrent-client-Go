package p2p

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0x40, 0}

	if err := writeMessage(&buf, msgRequest, payload); err != nil {
		t.Fatal(err)
	}
	msg, err := readMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ID != msgRequest {
		t.Errorf("message id = %d, want %d", msg.ID, msgRequest)
	}
	if !bytes.Equal(msg.Payload, payload) {
		t.Errorf("payload = %x, want %x", msg.Payload, payload)
	}
}

func TestReadKeepAlive(t *testing.T) {
	msg, err := readMessage(bytes.NewReader([]byte{0, 0, 0, 0}))
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Errorf("keep-alive returned %+v, want nil", msg)
	}
}

func TestReadMessageRejectsOversizedPayload(t *testing.T) {
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], maxMessageSize+1)

	if _, err := readMessage(bytes.NewReader(header[:])); err == nil {
		t.Fatal("expected an error for an oversized message")
	}
}

func TestHandshake(t *testing.T) {
	infoHash := [20]byte{1, 2, 3}
	peerID := [20]byte{4, 5, 6}

	client, remote := net.Pipe()
	defer client.Close()

	remoteErr := make(chan error, 1)
	go func() {
		defer remote.Close()

		req := make([]byte, handshakeLen)
		if _, err := io.ReadFull(remote, req); err != nil {
			remoteErr <- err
			return
		}
		if string(req[1:20]) != protocolName {
			remoteErr <- fmt.Errorf("protocol name = %q", string(req[1:20]))
			return
		}
		if !bytes.Equal(req[28:48], infoHash[:]) {
			remoteErr <- fmt.Errorf("info hash = %x", req[28:48])
			return
		}

		resp := make([]byte, handshakeLen)
		resp[0] = byte(len(protocolName))
		copy(resp[1:20], protocolName)
		copy(resp[28:48], infoHash[:])
		copy(resp[48:68], peerID[:])
		_, err := remote.Write(resp)
		remoteErr <- err
	}()

	if err := handshake(client, infoHash, peerID); err != nil {
		t.Fatal(err)
	}
	if err := <-remoteErr; err != nil {
		t.Fatal(err)
	}
}

func TestHandshakeRejectsWrongInfoHash(t *testing.T) {
	infoHash := [20]byte{1, 2, 3}
	wrongHash := [20]byte{9, 9, 9}

	client, remote := net.Pipe()
	defer client.Close()

	go func() {
		defer remote.Close()
		io.Copy(io.Discard, io.LimitReader(remote, handshakeLen))

		resp := make([]byte, handshakeLen)
		resp[0] = byte(len(protocolName))
		copy(resp[1:20], protocolName)
		copy(resp[28:48], wrongHash[:])
		remote.Write(resp)
	}()

	if err := handshake(client, infoHash, [20]byte{}); err == nil {
		t.Fatal("expected the wrong info hash to be rejected")
	}
}

func TestBitfield(t *testing.T) {
	b := newBitfield(16)
	for _, index := range []int{0, 7, 8, 15} {
		b.setPiece(index)
	}

	for index := 0; index < 16; index++ {
		want := index == 0 || index == 7 || index == 8 || index == 15
		if got := b.hasPiece(index); got != want {
			t.Errorf("hasPiece(%d) = %v, want %v", index, got, want)
		}
	}
	if b.hasPiece(16) {
		t.Error("hasPiece(16) is true for a 16 piece bitfield")
	}
}
