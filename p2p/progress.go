package p2p

// Progress describes how far along a download is.
type Progress struct {
	BytesDone   int64
	BytesTotal  int64
	PiecesDone  int
	PiecesTotal int
	Peers       int // peers we are talking to right now
	PeersTotal  int // peers the tracker gave us
}

// Percent returns how much of the torrent has been downloaded.
func (p Progress) Percent() float64 {
	if p.BytesTotal <= 0 {
		return 0
	}
	return float64(p.BytesDone) / float64(p.BytesTotal) * 100
}
