package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/jackpal/bencode-go"
)

// Port is the port we announce to the tracker.
const Port uint16 = 6881

// File is one entry in a multi-file torrent.
type File struct {
	Path   []string
	Length int64
}

// TorrentFile holds the pieces of a .torrent file that the client needs.
type TorrentFile struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int64
	Length      int64
	Name        string
	Files       []File
}

type bencodeInfo struct {
	Pieces      string        `bencode:"pieces"`
	PieceLength int64         `bencode:"piece length"`
	Length      int64         `bencode:"length"`
	Name        string        `bencode:"name"`
	Files       []bencodeFile `bencode:"files"`
}

type bencodeFile struct {
	Length int64    `bencode:"length"`
	Path   []string `bencode:"path"`
}

type bencodeTorrent struct {
	Announce     string             `bencode:"announce"`
	AnnounceList [][]string         `bencode:"announce-list"`
	Info         bencode.RawMessage `bencode:"info"`
}

// Open parses a torrent file.
func Open(path string) (TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TorrentFile{}, err
	}

	var bto bencodeTorrent
	if err := bencode.Unmarshal(bytes.NewReader(data), &bto); err != nil {
		return TorrentFile{}, fmt.Errorf("parse torrent: %w", err)
	}
	if len(bto.Info) == 0 {
		return TorrentFile{}, fmt.Errorf("torrent has no info dictionary")
	}

	var info bencodeInfo
	if err := bencode.Unmarshal(bytes.NewReader(bto.Info), &info); err != nil {
		return TorrentFile{}, fmt.Errorf("parse info dictionary: %w", err)
	}
	return bto.toTorrentFile(info)
}

func (i *bencodeInfo) splitPieceHashes() ([][20]byte, error) {
	const hashLen = 20 // SHA-1 hash length
	buf := []byte(i.Pieces)
	if len(buf)%hashLen != 0 {
		return nil, fmt.Errorf("received malformed pieces of length %d", len(buf))
	}

	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)
	for j := 0; j < numHashes; j++ {
		copy(hashes[j][:], buf[j*hashLen:(j+1)*hashLen])
	}
	return hashes, nil
}

func (bto *bencodeTorrent) announceURL() string {
	if bto.Announce != "" {
		return bto.Announce
	}
	for _, tier := range bto.AnnounceList {
		for _, url := range tier {
			if url != "" {
				return url
			}
		}
	}
	return ""
}

func (bto *bencodeTorrent) toTorrentFile(info bencodeInfo) (TorrentFile, error) {
	if info.Name == "" {
		return TorrentFile{}, fmt.Errorf("torrent has no name")
	}
	if info.PieceLength <= 0 {
		return TorrentFile{}, fmt.Errorf("torrent has invalid piece length %d", info.PieceLength)
	}

	announce := bto.announceURL()
	if announce == "" {
		return TorrentFile{}, fmt.Errorf("torrent has no announce URL")
	}

	var length int64
	var files []File
	if len(info.Files) > 0 {
		for _, f := range info.Files {
			cleaned, err := cleanPath(f.Path)
			if err != nil {
				return TorrentFile{}, err
			}
			files = append(files, File{Path: cleaned, Length: f.Length})
			length += f.Length
		}
	} else {
		length = info.Length
	}
	if length <= 0 {
		return TorrentFile{}, fmt.Errorf("torrent has no data")
	}

	pieceHashes, err := info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, err
	}
	wantPieces := (length + info.PieceLength - 1) / info.PieceLength
	if int64(len(pieceHashes)) != wantPieces {
		return TorrentFile{}, fmt.Errorf("torrent has %d piece hashes, expected %d", len(pieceHashes), wantPieces)
	}

	return TorrentFile{
		Announce:    announce,
		InfoHash:    sha1.Sum(bto.Info),
		PieceHashes: pieceHashes,
		PieceLength: info.PieceLength,
		Length:      length,
		Name:        info.Name,
		Files:       files,
	}, nil
}
