package torrentfile

import (
	"crypto/sha1"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTorrent(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.torrent")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenHashesRawInfoDict(t *testing.T) {
	info := "d6:lengthi5e4:name4:test12:piece lengthi5e6:pieces20:01234567890123456789e"
	path := writeTorrent(t, "d8:announce19:http://tracker.test4:info"+info+"e")

	got, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.InfoHash != sha1.Sum([]byte(info)) {
		t.Errorf("info hash = %x, want hash of the raw info dict", got.InfoHash)
	}
	if got.Name != "test" {
		t.Errorf("name = %q, want %q", got.Name, "test")
	}
	if got.Length != 5 || got.PieceLength != 5 {
		t.Errorf("length/piece length = %d/%d, want 5/5", got.Length, got.PieceLength)
	}
	if len(got.PieceHashes) != 1 {
		t.Errorf("got %d piece hashes, want 1", len(got.PieceHashes))
	}
	if got.Announce != "http://tracker.test" {
		t.Errorf("announce = %q", got.Announce)
	}
}

func TestOpenMultiFileAndAnnounceList(t *testing.T) {
	pieces := strings.Repeat("a", 40)
	info := "d5:filesl" +
		"d6:lengthi3e4:pathl1:aee" +
		"d6:lengthi5e4:pathl1:b1:cee" +
		"e4:name3:dir12:piece lengthi4e6:pieces40:" + pieces + "e"
	path := writeTorrent(t, "d13:announce-listll19:http://tracker.testee4:info"+info+"e")

	got, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Length != 8 {
		t.Errorf("length = %d, want 8", got.Length)
	}
	if len(got.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(got.Files))
	}
	if got.Files[1].Length != 5 || strings.Join(got.Files[1].Path, "/") != "b/c" {
		t.Errorf("second file = %+v, want length 5 and path b/c", got.Files[1])
	}
	if len(got.PieceHashes) != 2 {
		t.Errorf("got %d piece hashes, want 2", len(got.PieceHashes))
	}
	if got.Announce != "http://tracker.test" {
		t.Errorf("announce = %q, want fallback from announce-list", got.Announce)
	}
}

func TestOpenRejectsTruncatedTorrent(t *testing.T) {
	path := writeTorrent(t, "d8:announce19:http://tracker.test4:infod4:name4:test")
	if _, err := Open(path); err == nil {
		t.Fatal("expected an error for a truncated torrent")
	}
}

func TestOpenRejectsUnsafeFilePath(t *testing.T) {
	pieces := strings.Repeat("a", 20)
	info := "d5:filesl" +
		"d6:lengthi5e4:pathl2:..4:evilee" +
		"e4:name3:dir12:piece lengthi5e6:pieces20:" + pieces + "e"
	path := writeTorrent(t, "d8:announce19:http://tracker.test4:info"+info+"e")

	if _, err := Open(path); err == nil {
		t.Fatal("expected a torrent with a .. path to be rejected")
	}
}
