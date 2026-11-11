package torrentfile

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"bittorrent-client-go/p2p"
	"bittorrent-client-go/tracker"
)

// DownloadToFile downloads the torrent and writes it to path. Torrents with
// more than one file need a directory, use DownloadToDir for those.
//
// The progress function, if it is not nil, is called about once a second with
// the state of the download.
func (t *TorrentFile) DownloadToFile(ctx context.Context, path string, progress func(p2p.Progress)) error {
	if len(t.Files) > 0 {
		return fmt.Errorf("this torrent has %d files, use DownloadToDir instead", len(t.Files))
	}
	return t.download(ctx, path, progress)
}

// DownloadToDir downloads the torrent into dir. Everything is written to a
// folder inside dir that is named after the torrent.
func (t *TorrentFile) DownloadToDir(ctx context.Context, dir string, progress func(p2p.Progress)) error {
	return t.download(ctx, filepath.Join(dir, t.Name), progress)
}

func (t *TorrentFile) download(ctx context.Context, path string, progress func(p2p.Progress)) error {
	peerID, err := randomPeerID()
	if err != nil {
		return err
	}

	response, err := tracker.Announce(ctx, t.Announce, t.InfoHash, peerID, Port, t.Length)
	if err != nil {
		return fmt.Errorf("announce to tracker: %w", err)
	}
	if len(response.Peers) == 0 {
		return fmt.Errorf("the tracker did not give us any peers")
	}

	writer, cleanup, err := t.openOutput(path)
	if err != nil {
		return err
	}
	defer cleanup()

	torrent := p2p.Torrent{
		Peers:       response.Peers,
		PeerID:      peerID,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		Name:        t.Name,
	}
	return torrent.Download(ctx, writer, progress)
}

// openOutput creates the file or files we are going to write to. The
// returned writer takes the offset of every byte inside the torrent.
func (t *TorrentFile) openOutput(path string) (io.WriterAt, func(), error) {
	if len(t.Files) == 0 {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			return nil, nil, err
		}
		// Reserve the space so we do not have to grow the file later.
		if err := file.Truncate(t.Length); err != nil {
			file.Close()
			return nil, nil, err
		}
		return file, func() { file.Close() }, nil
	}

	writer := &multiFileWriter{}
	cleanup := func() {
		for _, part := range writer.parts {
			part.file.Close()
		}
	}

	var offset int64
	for _, f := range t.Files {
		fullPath, err := safeJoin(path, f.Path)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			cleanup()
			return nil, nil, err
		}

		file, err := os.OpenFile(fullPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if err := file.Truncate(f.Length); err != nil {
			file.Close()
			cleanup()
			return nil, nil, err
		}

		writer.parts = append(writer.parts, filePart{offset: offset, length: f.Length, file: file})
		offset += f.Length
	}
	return writer, cleanup, nil
}

func randomPeerID() ([20]byte, error) {
	var id [20]byte
	if _, err := rand.Read(id[:]); err != nil {
		return id, err
	}
	// The first eight bytes identify the client, the rest is random.
	copy(id[:8], "-GO0001-")
	return id, nil
}

// filePart is one file of a multi file torrent.
type filePart struct {
	offset int64 // where the file starts inside the torrent
	length int64
	file   *os.File
}

// multiFileWriter spreads the torrent data over several files.
type multiFileWriter struct {
	parts []filePart
}

func (w *multiFileWriter) WriteAt(p []byte, off int64) (int, error) {
	written := 0
	for len(p) > 0 {
		part := w.partAt(off + int64(written))
		if part == nil {
			return written, fmt.Errorf("no file for offset %d", off+int64(written))
		}

		start := off + int64(written)
		chunk := part.offset + part.length - start
		if chunk > int64(len(p)) {
			chunk = int64(len(p))
		}

		n, err := part.file.WriteAt(p[:chunk], start-part.offset)
		written += n
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
		p = p[n:]
	}
	return written, nil
}

func (w *multiFileWriter) partAt(offset int64) *filePart {
	i := sort.Search(len(w.parts), func(i int) bool {
		return w.parts[i].offset > offset
	}) - 1
	if i < 0 {
		return nil
	}

	part := &w.parts[i]
	if offset >= part.offset+part.length {
		return nil
	}
	return part
}
