package p2p

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func TestPieceManagerHandsOutOnePieceAtATime(t *testing.T) {
	hashes := [][20]byte{{1}, {2}, {3}}
	pm, err := newPieceManager(4, 10, hashes)
	if err != nil {
		t.Fatal(err)
	}

	has := newBitfield(3)
	has.setPiece(1)

	p, ok := pm.next(has)
	if !ok || p.index != 1 {
		t.Fatalf("next() = %v, %v; want piece 1", p, ok)
	}
	if p.length != 4 {
		t.Errorf("piece length = %d, want 4", p.length)
	}
	if _, ok := pm.next(has); ok {
		t.Error("the same piece was handed out twice")
	}

	pm.release(p)
	p, ok = pm.next(has)
	if !ok || p.index != 1 {
		t.Fatalf("next() after release = %v, %v; want piece 1", p, ok)
	}

	pm.finish(p)
	if pm.isDone() {
		t.Error("the torrent is done after a single piece")
	}
	if pm.doneCount() != 1 {
		t.Errorf("done count = %d, want 1", pm.doneCount())
	}
}

func TestPieceManagerRejectsMismatchedLength(t *testing.T) {
	hashes := [][20]byte{{1}, {2}, {3}}
	if _, err := newPieceManager(4, 100, hashes); err == nil {
		t.Fatal("expected an error for a length that does not match the pieces")
	}
}

// fakePeer is a very small peer that only knows how to serve one torrent.
type fakePeer struct {
	listener     net.Listener
	infoHash     [20]byte
	data         []byte
	pieceLen     int64
	lateBitfield bool
	err          chan error
}

func newFakePeerWithOptions(t *testing.T, infoHash [20]byte, data []byte, pieceLen int64, lateBitfield bool) *fakePeer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	p := &fakePeer{
		listener:     listener,
		infoHash:     infoHash,
		data:         data,
		pieceLen:     pieceLen,
		lateBitfield: lateBitfield,
		err:          make(chan error, 1),
	}
	go p.serve()
	t.Cleanup(func() { listener.Close() })
	return p
}

func (p *fakePeer) peer() Peer {
	addr := p.listener.Addr().(*net.TCPAddr)
	return Peer{IP: addr.IP, Port: uint16(addr.Port)}
}

func (p *fakePeer) serve() {
	conn, err := p.listener.Accept()
	if err != nil {
		p.err <- err
		return
	}
	defer conn.Close()

	req := make([]byte, handshakeLen)
	if _, err := io.ReadFull(conn, req); err != nil {
		p.err <- err
		return
	}

	resp := make([]byte, handshakeLen)
	resp[0] = byte(len(protocolName))
	copy(resp[1:20], protocolName)
	copy(resp[28:48], p.infoHash[:])
	if _, err := conn.Write(resp); err != nil {
		p.err <- err
		return
	}

	// Tell the client which pieces we have and unchoke it. Some peers send
	// the bitfield after the unchoke, so both orders have to work.
	pieceCount := int((int64(len(p.data)) + p.pieceLen - 1) / p.pieceLen)
	has := newBitfield(pieceCount)
	for i := 0; i < pieceCount; i++ {
		has.setPiece(i)
	}

	sendBitfield := func() error {
		return writeMessage(conn, msgBitfield, has)
	}
	sendUnchoke := func() error {
		return writeMessage(conn, msgUnchoke, nil)
	}

	if p.lateBitfield {
		if err := sendUnchoke(); err != nil {
			p.err <- err
			return
		}
		if err := sendBitfield(); err != nil {
			p.err <- err
			return
		}
	} else {
		if err := sendBitfield(); err != nil {
			p.err <- err
			return
		}
		if err := sendUnchoke(); err != nil {
			p.err <- err
			return
		}
	}

	for {
		msg, err := readMessage(conn)
		if err != nil {
			p.err <- nil // the client hung up, which is how it says goodbye
			return
		}
		if msg == nil || msg.ID != msgRequest {
			continue
		}

		index := binary.BigEndian.Uint32(msg.Payload[0:4])
		begin := binary.BigEndian.Uint32(msg.Payload[4:8])
		length := binary.BigEndian.Uint32(msg.Payload[8:12])

		start := int64(index)*p.pieceLen + int64(begin)
		block := p.data[start : start+int64(length)]

		payload := make([]byte, 8+len(block))
		binary.BigEndian.PutUint32(payload[0:4], index)
		binary.BigEndian.PutUint32(payload[4:8], begin)
		copy(payload[8:], block)

		if err := writeMessage(conn, msgPiece, payload); err != nil {
			p.err <- err
			return
		}
	}
}

func TestDownloadFromPeer(t *testing.T) {
	runDownloadTest(t, false)
}

func TestDownloadWhenBitfieldArrivesLate(t *testing.T) {
	runDownloadTest(t, true)
}

func runDownloadTest(t *testing.T, lateBitfield bool) {
	data := []byte("hello world, this is a tiny torrent")
	const pieceLen = 8

	var hashes [][20]byte
	for offset := int64(0); offset < int64(len(data)); offset += pieceLen {
		end := offset + pieceLen
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		hashes = append(hashes, sha1.Sum(data[offset:end]))
	}

	infoHash := [20]byte{1, 2, 3, 4}
	peer := newFakePeerWithOptions(t, infoHash, data, pieceLen, lateBitfield)

	torrent := Torrent{
		Peers:       []Peer{peer.peer()},
		InfoHash:    infoHash,
		PieceHashes: hashes,
		PieceLength: pieceLen,
		Length:      int64(len(data)),
	}

	out, err := os.CreateTemp(t.TempDir(), "downloaded")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := torrent.Download(ctx, out); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Errorf("downloaded %q, want %q", got, data)
	}

	select {
	case err := <-peer.err:
		if err != nil {
			t.Errorf("the peer ran into trouble: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("the peer did not stop")
	}
}

func TestDownloadWithoutPeers(t *testing.T) {
	torrent := Torrent{
		InfoHash:    [20]byte{1},
		PieceHashes: [][20]byte{{1}},
		PieceLength: 4,
		Length:      4,
	}
	if err := torrent.Download(context.Background(), nil); err == nil {
		t.Fatal("expected an error when there are no peers")
	}
}
