package p2p

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	blockSize          = 16 * 1024
	dialTimeout        = 10 * time.Second
	handshakeTimeout   = 15 * time.Second
	readTimeout        = 60 * time.Second
	writeTimeout       = 60 * time.Second
	chokeTimeout       = 30 * time.Second
	maxPendingRequests = 5
)

var errChoked = errors.New("peer choked us")

func (t *Torrent) downloadFromPeer(ctx context.Context, peer Peer, pm *pieceManager, out io.WriterAt, addConn func(net.Conn) bool) error {
	dialer := net.Dialer{Timeout: dialTimeout}
	c, err := dialer.DialContext(ctx, "tcp", peer.String())
	if err != nil {
		return fmt.Errorf("connect to %s: %w", peer, err)
	}
	defer c.Close()

	if !addConn(c) {
		return nil // another worker finished the torrent while we were dialing
	}

	c.SetDeadline(time.Now().Add(handshakeTimeout))
	if err := handshake(c, t.InfoHash, t.PeerID); err != nil {
		return fmt.Errorf("%s: %w", peer, err)
	}
	// We have no pieces, but we still send an empty bitfield. Peers treat a
	// client that never sends one as a spider and refuse to unchoke us.
	if err := writeMessage(c, msgBitfield, make([]byte, (len(t.PieceHashes)+7)/8)); err != nil {
		return fmt.Errorf("%s: send bitfield: %w", peer, err)
	}
	if err := writeMessage(c, msgInterested, nil); err != nil {
		return fmt.Errorf("%s: send interested: %w", peer, err)
	}
	c.SetDeadline(time.Time{})

	has := newBitfield(len(t.PieceHashes))
	choked := true

	for {
		if ctx.Err() != nil {
			return nil
		}

		if !choked {
			if p, ok := pm.next(has); ok {
				err := t.downloadPiece(ctx, c, p, has, out)
				if errors.Is(err, errChoked) {
					pm.release(p)
					choked = true
					continue
				}
				if err != nil {
					pm.release(p)
					return fmt.Errorf("%s: piece %d: %w", peer, p.index, err)
				}
				pm.finish(p)
				continue
			}
			if pm.isDone() {
				return nil
			}
		}

		// We have nothing to download from this peer right now, so listen
		// for the messages it sends us. Peers are allowed to send their
		// bitfield after they unchoke us.
		c.SetReadDeadline(time.Now().Add(chokeTimeout))
		msg, err := readMessage(c)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%s: read message: %w", peer, err)
		}
		if msg == nil {
			continue // keep-alive
		}

		switch msg.ID {
		case msgUnchoke:
			choked = false
		case msgChoke:
			choked = true
		case msgHave, msgBitfield:
			t.notePeerMessage(has, msg)
		}
	}
}

func (t *Torrent) downloadPiece(ctx context.Context, c net.Conn, p piece, has bitfield, out io.WriterAt) error {
	buf := make([]byte, p.length)
	var nextOffset int64
	var received int64
	pending := 0

	for received < p.length {
		if err := ctx.Err(); err != nil {
			return err
		}

		for pending < maxPendingRequests && nextOffset < p.length {
			length := int64(blockSize)
			if remaining := p.length - nextOffset; remaining < length {
				length = remaining
			}

			req := make([]byte, 12)
			binary.BigEndian.PutUint32(req[0:4], uint32(p.index))
			binary.BigEndian.PutUint32(req[4:8], uint32(nextOffset))
			binary.BigEndian.PutUint32(req[8:12], uint32(length))

			c.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := writeMessage(c, msgRequest, req); err != nil {
				return err
			}
			pending++
			nextOffset += length
		}

		c.SetReadDeadline(time.Now().Add(readTimeout))
		msg, err := readMessage(c)
		if err != nil {
			return err
		}
		if msg == nil {
			continue // keep-alive
		}

		switch msg.ID {
		case msgChoke:
			return errChoked
		case msgHave, msgBitfield:
			t.notePeerMessage(has, msg)
		case msgPiece:
			if len(msg.Payload) < 8 {
				return fmt.Errorf("piece message is too short")
			}
			index := binary.BigEndian.Uint32(msg.Payload[0:4])
			begin := binary.BigEndian.Uint32(msg.Payload[4:8])
			block := msg.Payload[8:]

			if int(index) != p.index {
				return fmt.Errorf("got piece %d while downloading piece %d", index, p.index)
			}
			if int64(begin)+int64(len(block)) > p.length {
				return fmt.Errorf("block runs past the end of the piece")
			}

			copy(buf[begin:], block)
			pending--
			received += int64(len(block))
		}
	}

	if sha1.Sum(buf) != p.hash {
		return fmt.Errorf("hash mismatch")
	}
	if _, err := out.WriteAt(buf, int64(p.index)*t.PieceLength); err != nil {
		return fmt.Errorf("write piece: %w", err)
	}
	return nil
}

// notePeerMessage keeps our idea of the peer's piece list up to date.
func (t *Torrent) notePeerMessage(has bitfield, msg *Message) {
	switch msg.ID {
	case msgHave:
		if len(msg.Payload) != 4 {
			return
		}
		index := int(binary.BigEndian.Uint32(msg.Payload))
		if index >= 0 && index < len(t.PieceHashes) {
			has.setPiece(index)
		}
	case msgBitfield:
		copy(has, msg.Payload)
	}
}
