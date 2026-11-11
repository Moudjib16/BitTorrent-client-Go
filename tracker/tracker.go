package tracker

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"

	"bittorrent-client-go/p2p"
)

const (
	requestTimeout  = 15 * time.Second
	maxResponseSize = 4 << 20
)

// Response is what we need from a tracker announce.
type Response struct {
	Interval time.Duration
	Peers    []p2p.Peer
}

type bencodeResponse struct {
	FailureReason string             `bencode:"failure reason"`
	Interval      int64              `bencode:"interval"`
	Peers         bencode.RawMessage `bencode:"peers"`
}

type bencodePeer struct {
	IP   string `bencode:"ip"`
	Port int    `bencode:"port"`
}

// Announce asks the tracker for peers. Only HTTP trackers are supported for
// now; UDP trackers have a completely different protocol.
func Announce(ctx context.Context, announceURL string, infoHash, peerID [20]byte, port uint16, left int64) (*Response, error) {
	u, err := url.Parse(announceURL)
	if err != nil {
		return nil, fmt.Errorf("parse announce URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported tracker scheme %q, only HTTP trackers work right now", u.Scheme)
	}

	query := u.Query()
	query.Set("info_hash", string(infoHash[:]))
	query.Set("peer_id", string(peerID[:]))
	query.Set("port", strconv.Itoa(int(port)))
	query.Set("uploaded", "0")
	query.Set("downloaded", "0")
	query.Set("left", strconv.FormatInt(left, 10))
	query.Set("compact", "1")
	query.Set("numwant", "50")
	query.Set("event", "started")
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "bittorrent-client-go/0.1")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact tracker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("read tracker response: %w", err)
	}
	return parseResponse(body)
}

func parseResponse(body []byte) (*Response, error) {
	var raw bencodeResponse
	if err := bencode.Unmarshal(bytes.NewReader(body), &raw); err != nil {
		return nil, fmt.Errorf("parse tracker response: %w", err)
	}
	if raw.FailureReason != "" {
		return nil, fmt.Errorf("tracker: %s", raw.FailureReason)
	}

	peers, err := parsePeers(raw.Peers)
	if err != nil {
		return nil, err
	}

	interval := time.Duration(raw.Interval) * time.Second
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	return &Response{Interval: interval, Peers: peers}, nil
}

// parsePeers handles both the compact format and the older list of dicts.
func parsePeers(raw bencode.RawMessage) ([]p2p.Peer, error) {
	if len(raw) == 0 || string(raw) == "0:" {
		return nil, nil
	}

	if raw[0] == 'l' {
		return parsePeerList(raw)
	}

	var compact string
	if err := bencode.Unmarshal(bytes.NewReader(raw), &compact); err != nil {
		return nil, fmt.Errorf("parse peers: %w", err)
	}
	return parseCompactPeers([]byte(compact))
}

func parseCompactPeers(buf []byte) ([]p2p.Peer, error) {
	const peerSize = 6 // 4 bytes IPv4 + 2 bytes port
	if len(buf)%peerSize != 0 {
		return nil, fmt.Errorf("compact peer list has %d bytes, expected a multiple of %d", len(buf), peerSize)
	}

	peers := make([]p2p.Peer, 0, len(buf)/peerSize)
	for i := 0; i < len(buf); i += peerSize {
		ip := net.IPv4(buf[i], buf[i+1], buf[i+2], buf[i+3])
		port := binary.BigEndian.Uint16(buf[i+4 : i+6])
		if port == 0 {
			continue
		}
		peers = append(peers, p2p.Peer{IP: ip, Port: port})
	}
	return peers, nil
}

func parsePeerList(raw bencode.RawMessage) ([]p2p.Peer, error) {
	var list []bencodePeer
	if err := bencode.Unmarshal(bytes.NewReader(raw), &list); err != nil {
		return nil, fmt.Errorf("parse peer list: %w", err)
	}

	peers := make([]p2p.Peer, 0, len(list))
	for _, p := range list {
		ip := net.ParseIP(p.IP)
		if ip == nil || p.Port <= 0 || p.Port > 65535 {
			continue
		}
		peers = append(peers, p2p.Peer{IP: ip, Port: uint16(p.Port)})
	}
	return peers, nil
}
