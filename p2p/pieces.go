package p2p

import (
	"fmt"
	"sync"
)

type piece struct {
	index  int
	hash   [20]byte
	length int64
}

type pieceStatus int

const (
	piecePending pieceStatus = iota
	pieceInProgress
	pieceDone
)

// pieceManager hands out pieces to workers and keeps track of what is left.
type pieceManager struct {
	mu        sync.Mutex
	pieces    []piece
	status    []pieceStatus
	done      int
	doneBytes int64
	totalSize int64
}

func newPieceManager(pieceLength, totalLength int64, hashes [][20]byte) (*pieceManager, error) {
	if pieceLength <= 0 {
		return nil, fmt.Errorf("invalid piece length %d", pieceLength)
	}
	if len(hashes) == 0 {
		return nil, fmt.Errorf("torrent has no pieces")
	}
	if totalLength <= pieceLength*int64(len(hashes)-1) || totalLength > pieceLength*int64(len(hashes)) {
		return nil, fmt.Errorf("torrent length %d does not match %d pieces of %d bytes", totalLength, len(hashes), pieceLength)
	}

	pieces := make([]piece, len(hashes))
	for i, hash := range hashes {
		length := pieceLength
		if i == len(hashes)-1 {
			length = totalLength - int64(i)*pieceLength
		}
		pieces[i] = piece{index: i, hash: hash, length: length}
	}

	return &pieceManager{
		pieces:    pieces,
		status:    make([]pieceStatus, len(pieces)),
		totalSize: totalLength,
	}, nil
}

// next claims the first piece this peer has that nobody is downloading yet.
func (pm *pieceManager) next(has bitfield) (piece, bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i := range pm.pieces {
		if pm.status[i] == piecePending && has.hasPiece(i) {
			pm.status[i] = pieceInProgress
			return pm.pieces[i], true
		}
	}
	return piece{}, false
}

// release puts a piece back in the queue after a failed download.
func (pm *pieceManager) release(p piece) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.status[p.index] == pieceInProgress {
		pm.status[p.index] = piecePending
	}
}

func (pm *pieceManager) finish(p piece) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.status[p.index] != pieceDone {
		pm.status[p.index] = pieceDone
		pm.done++
		pm.doneBytes += p.length
	}
}

// progress returns a snapshot of the download for the status line.
func (pm *pieceManager) progress(peers, peersTotal int) Progress {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return Progress{
		BytesDone:   pm.doneBytes,
		BytesTotal:  pm.totalSize,
		PiecesDone:  pm.done,
		PiecesTotal: len(pm.pieces),
		Peers:       peers,
		PeersTotal:  peersTotal,
	}
}

func (pm *pieceManager) isDone() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.done == len(pm.pieces)
}

func (pm *pieceManager) doneCount() int {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.done
}

func (pm *pieceManager) total() int {
	return len(pm.pieces)
}
