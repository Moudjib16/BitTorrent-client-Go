package p2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
)

// Torrent is everything needed to download one torrent.
type Torrent struct {
	Peers       []Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int64
	Length      int64
	Name        string
}

// Download fetches every piece and writes it to out. The writer must be safe
// to use from multiple goroutines, since each peer writes its own pieces.
func (t *Torrent) Download(ctx context.Context, out io.WriterAt) error {
	if len(t.Peers) == 0 {
		return fmt.Errorf("no peers to download from")
	}

	pm, err := newPieceManager(t.PieceLength, t.Length, t.PieceHashes)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var connMu sync.Mutex
	var conns []net.Conn
	closeConns := func() {
		connMu.Lock()
		defer connMu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	}
	go func() {
		<-ctx.Done()
		closeConns()
	}()

	// addConn returns false if the download is already finished.
	addConn := func(c net.Conn) bool {
		connMu.Lock()
		defer connMu.Unlock()
		if ctx.Err() != nil {
			return false
		}
		conns = append(conns, c)
		return true
	}

	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for _, peer := range t.Peers {
		wg.Add(1)
		go func(p Peer) {
			defer wg.Done()

			err := t.downloadFromPeer(ctx, p, pm, out, addConn)
			if pm.isDone() {
				cancel()
				return
			}
			if err != nil && ctx.Err() == nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
			}
		}(peer)
	}
	wg.Wait()

	if !pm.isDone() {
		errMu.Lock()
		defer errMu.Unlock()
		if firstErr != nil {
			return fmt.Errorf("download incomplete (%d of %d pieces): %w", pm.doneCount(), pm.total(), firstErr)
		}
		return fmt.Errorf("download incomplete (%d of %d pieces)", pm.doneCount(), pm.total())
	}
	return nil
}
