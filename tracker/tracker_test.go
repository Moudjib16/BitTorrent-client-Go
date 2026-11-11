package tracker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestAnnounceSendsBinaryInfoHash(t *testing.T) {
	infoHash := [20]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x20, 0x7f, 0x80, 0x01}
	peerID := [20]byte{'T', 'E', 'S', 'T'}
	compact := []byte{127, 0, 0, 1, 0x1a, 0xe1}

	var received url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.URL.Query()
		w.Write([]byte("d8:intervali1800e5:peers6:" + string(compact) + "e"))
	}))
	defer server.Close()

	resp, err := Announce(context.Background(), server.URL+"/announce", infoHash, peerID, 6881, 1234)
	if err != nil {
		t.Fatal(err)
	}

	if got := received.Get("info_hash"); got != string(infoHash[:]) {
		t.Errorf("info_hash decoded to %x, want %x", got, infoHash)
	}
	if got := received.Get("peer_id"); got != string(peerID[:]) {
		t.Errorf("peer_id decoded to %q, want %q", got, peerID)
	}
	if got := received.Get("left"); got != "1234" {
		t.Errorf("left = %q, want 1234", got)
	}
	if got := received.Get("port"); got != "6881" {
		t.Errorf("port = %q, want 6881", got)
	}

	if resp.Interval != 30*time.Minute {
		t.Errorf("interval = %v, want 30m", resp.Interval)
	}
	if len(resp.Peers) != 1 || resp.Peers[0].String() != "127.0.0.1:6881" {
		t.Errorf("peers = %v, want [127.0.0.1:6881]", resp.Peers)
	}
}

func TestParseNonCompactPeers(t *testing.T) {
	body := "d8:intervali60e5:peersld2:ip9:127.0.0.14:porti6881eeee"
	resp, err := parseResponse([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Interval != time.Minute {
		t.Errorf("interval = %v, want 1m", resp.Interval)
	}
	if len(resp.Peers) != 1 || resp.Peers[0].String() != "127.0.0.1:6881" {
		t.Errorf("peers = %v, want [127.0.0.1:6881]", resp.Peers)
	}
}

func TestParseTrackerFailure(t *testing.T) {
	body := "d14:failure reason9:not founde"
	if _, err := parseResponse([]byte(body)); err == nil {
		t.Fatal("expected the tracker failure reason to be returned")
	}
}

func TestParseCompactPeersRejectsBadLength(t *testing.T) {
	if _, err := parseCompactPeers([]byte{1, 2, 3, 4, 5}); err == nil {
		t.Fatal("expected an error for a truncated compact peer")
	}
}

func TestAnnounceRejectsUDPTrackers(t *testing.T) {
	_, err := Announce(context.Background(), "udp://tracker.test:6969/announce", [20]byte{}, [20]byte{}, 6881, 1)
	if err == nil {
		t.Fatal("expected UDP trackers to be rejected")
	}
}
