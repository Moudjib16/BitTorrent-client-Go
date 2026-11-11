package torrentfile

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMultiFileTorrentIsSplitOverFiles(t *testing.T) {
	torrent := TorrentFile{
		Name:   "release",
		Length: 15,
		Files: []File{
			{Path: []string{"a.bin"}, Length: 10},
			{Path: []string{"sub", "b.bin"}, Length: 5},
		},
	}

	root := filepath.Join(t.TempDir(), "release")
	writer, cleanup, err := torrent.openOutput(root)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	data := []byte("0123456789abcde")
	if _, err := writer.WriteAt(data, 0); err != nil {
		t.Fatal(err)
	}

	assertFile(t, filepath.Join(root, "a.bin"), "0123456789")
	assertFile(t, filepath.Join(root, "sub", "b.bin"), "abcde")
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s contains %q, want %q", path, got, want)
	}
}

func TestOpenOutputRejectsPathTraversal(t *testing.T) {
	torrent := TorrentFile{
		Name:   "evil",
		Length: 5,
		Files: []File{
			{Path: []string{"..", "..", "escaped.txt"}, Length: 5},
		},
	}

	root := filepath.Join(t.TempDir(), "nested", "evil")
	if _, _, err := torrent.openOutput(root); err == nil {
		t.Fatal("expected the unsafe path to be rejected")
	}
}

// TestDownloadToFile runs the whole thing: tracker announce, peer handshake
// and the download itself.
func TestDownloadToFile(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog")
	const pieceLen = 16

	var hashes [][20]byte
	for offset := int64(0); offset < int64(len(data)); offset += pieceLen {
		end := offset + pieceLen
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		hashes = append(hashes, sha1.Sum(data[offset:end]))
	}

	infoHash := [20]byte{9, 8, 7, 6}
	peerPort := startFakePeer(t, infoHash, data, pieceLen)

	tracker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compact := []byte{127, 0, 0, 1, byte(peerPort >> 8), byte(peerPort)}
		w.Write([]byte("d8:intervali1800e5:peers6:" + string(compact) + "e"))
	}))
	defer tracker.Close()

	torrent := TorrentFile{
		Announce:    tracker.URL,
		InfoHash:    infoHash,
		PieceHashes: hashes,
		PieceLength: pieceLen,
		Length:      int64(len(data)),
		Name:        "fox.txt",
	}

	output := filepath.Join(t.TempDir(), "fox.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := torrent.DownloadToFile(ctx, output, nil); err != nil {
		t.Fatal(err)
	}
	assertFile(t, output, string(data))
}

// startFakePeer runs a peer that serves one torrent and returns its port.
func startFakePeer(t *testing.T, infoHash [20]byte, data []byte, pieceLen int64) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		req := make([]byte, 68)
		if _, err := io.ReadFull(conn, req); err != nil {
			return
		}
		resp := make([]byte, 68)
		resp[0] = 19
		copy(resp[1:20], "BitTorrent protocol")
		copy(resp[28:48], infoHash[:])
		if _, err := conn.Write(resp); err != nil {
			return
		}

		pieceCount := int((int64(len(data)) + pieceLen - 1) / pieceLen)
		bitfield := make([]byte, (pieceCount+7)/8)
		for i := 0; i < pieceCount; i++ {
			bitfield[i/8] |= 1 << (7 - uint(i%8))
		}
		writeTestMessage(conn, 5, bitfield) // bitfield
		writeTestMessage(conn, 1, nil)      // unchoke

		for {
			msg, err := readTestMessage(conn)
			if err != nil {
				return
			}
			if msg == nil || msg[0] != 6 { // request
				continue
			}
			payload := msg[1:]
			index := binary.BigEndian.Uint32(payload[0:4])
			begin := binary.BigEndian.Uint32(payload[4:8])
			length := binary.BigEndian.Uint32(payload[8:12])

			start := int64(index)*pieceLen + int64(begin)
			block := data[start : start+int64(length)]

			piece := make([]byte, 8+len(block))
			binary.BigEndian.PutUint32(piece[0:4], index)
			binary.BigEndian.PutUint32(piece[4:8], begin)
			copy(piece[8:], block)
			writeTestMessage(conn, 7, piece) // piece
		}
	}()

	return listener.Addr().(*net.TCPAddr).Port
}

func writeTestMessage(conn net.Conn, id byte, payload []byte) {
	buf := make([]byte, 5+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)+1))
	buf[4] = id
	copy(buf[5:], payload)
	conn.Write(buf)
}

func readTestMessage(conn net.Conn) ([]byte, error) {
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
