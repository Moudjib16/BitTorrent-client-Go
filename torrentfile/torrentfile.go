package torrentfile

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"strconv"

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
	Announce     string      `bencode:"announce"`
	AnnounceList [][]string  `bencode:"announce-list"`
	Info         bencodeInfo `bencode:"info"`
}

// Open parses a torrent file.
func Open(path string) (TorrentFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TorrentFile{}, err
	}

	bto := bencodeTorrent{}
	if err := bencode.Unmarshal(bytes.NewReader(data), &bto); err != nil {
		return TorrentFile{}, fmt.Errorf("parse torrent: %w", err)
	}

	info, err := findInfoDict(data)
	if err != nil {
		return TorrentFile{}, err
	}
	return bto.toTorrentFile(info)
}

// findInfoDict returns the exact bencoded bytes of the info dictionary. The
// info hash is a SHA-1 of these bytes, so re-encoding the dictionary is not
// good enough: any field we do not know about would change the hash.
func findInfoDict(data []byte) ([]byte, error) {
	if len(data) == 0 || data[0] != 'd' {
		return nil, fmt.Errorf("torrent is not a bencoded dictionary")
	}

	pos := 1
	for pos < len(data) && data[pos] != 'e' {
		key, next, err := readString(data, pos)
		if err != nil {
			return nil, err
		}
		valueEnd, err := skipValue(data, next)
		if err != nil {
			return nil, err
		}
		if string(key) == "info" {
			return data[next:valueEnd], nil
		}
		pos = valueEnd
	}
	return nil, fmt.Errorf("torrent has no info dictionary")
}

func readString(data []byte, pos int) ([]byte, int, error) {
	start := pos
	for pos < len(data) && data[pos] >= '0' && data[pos] <= '9' {
		pos++
	}
	if pos == start || pos >= len(data) || data[pos] != ':' {
		return nil, 0, fmt.Errorf("malformed bencode string at byte %d", start)
	}

	n, err := strconv.Atoi(string(data[start:pos]))
	if err != nil {
		return nil, 0, fmt.Errorf("malformed bencode string length: %w", err)
	}
	pos++ // skip ':'
	if n < 0 || pos+n > len(data) {
		return nil, 0, fmt.Errorf("bencode string at byte %d runs past end of file", start)
	}
	return data[pos : pos+n], pos + n, nil
}

func skipValue(data []byte, pos int) (int, error) {
	if pos >= len(data) {
		return 0, fmt.Errorf("unexpected end of bencode data")
	}

	switch data[pos] {
	case 'i':
		end := bytes.IndexByte(data[pos:], 'e')
		if end < 0 {
			return 0, fmt.Errorf("unterminated bencode integer at byte %d", pos)
		}
		return pos + end + 1, nil

	case 'l':
		pos++
		for {
			if pos >= len(data) {
				return 0, fmt.Errorf("unterminated bencode list")
			}
			if data[pos] == 'e' {
				return pos + 1, nil
			}
			next, err := skipValue(data, pos)
			if err != nil {
				return 0, err
			}
			pos = next
		}

	case 'd':
		pos++
		for {
			if pos >= len(data) {
				return 0, fmt.Errorf("unterminated bencode dictionary")
			}
			if data[pos] == 'e' {
				return pos + 1, nil
			}
			_, next, err := readString(data, pos)
			if err != nil {
				return 0, err
			}
			next, err = skipValue(data, next)
			if err != nil {
				return 0, err
			}
			pos = next
		}

	default:
		_, next, err := readString(data, pos)
		return next, err
	}
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

func (bto *bencodeTorrent) toTorrentFile(info []byte) (TorrentFile, error) {
	if bto.Info.Name == "" {
		return TorrentFile{}, fmt.Errorf("torrent has no name")
	}
	if bto.Info.PieceLength <= 0 {
		return TorrentFile{}, fmt.Errorf("torrent has invalid piece length %d", bto.Info.PieceLength)
	}

	announce := bto.announceURL()
	if announce == "" {
		return TorrentFile{}, fmt.Errorf("torrent has no announce URL")
	}

	var length int64
	var files []File
	if len(bto.Info.Files) > 0 {
		for _, f := range bto.Info.Files {
			if len(f.Path) == 0 {
				return TorrentFile{}, fmt.Errorf("torrent has a file with an empty path")
			}
			files = append(files, File{Path: f.Path, Length: f.Length})
			length += f.Length
		}
	} else {
		length = bto.Info.Length
	}
	if length <= 0 {
		return TorrentFile{}, fmt.Errorf("torrent has no data")
	}

	pieceHashes, err := bto.Info.splitPieceHashes()
	if err != nil {
		return TorrentFile{}, err
	}
	wantPieces := (length + bto.Info.PieceLength - 1) / bto.Info.PieceLength
	if int64(len(pieceHashes)) != wantPieces {
		return TorrentFile{}, fmt.Errorf("torrent has %d piece hashes, expected %d", len(pieceHashes), wantPieces)
	}

	return TorrentFile{
		Announce:    announce,
		InfoHash:    sha1.Sum(info),
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length:      length,
		Name:        bto.Info.Name,
		Files:       files,
	}, nil
}
