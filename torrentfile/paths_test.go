package torrentfile

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestCleanPathKeepsNormalPaths(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"a.txt"}, "a.txt"},
		{[]string{"sub", "b.txt"}, "sub/b.txt"},
		{[]string{"deep", "nested", "c.txt"}, "deep/nested/c.txt"},
		// Some torrents put the whole path in one component.
		{[]string{"a/b.txt"}, "a/b.txt"},
	}

	for _, test := range tests {
		got, err := cleanPath(test.parts)
		if err != nil {
			t.Errorf("cleanPath(%q) failed: %v", test.parts, err)
			continue
		}
		if joined := filepath.ToSlash(filepath.Join(got...)); joined != test.want {
			t.Errorf("cleanPath(%q) = %q, want %q", test.parts, joined, test.want)
		}
	}
}

func TestCleanPathRejectsEscapes(t *testing.T) {
	absolute := filepath.Join(t.TempDir(), "abs.txt")
	bad := [][]string{
		{"..", "evil"},
		{"a", "..", "..", "evil"},
		{"../../evil"},
		{absolute},
		{},
		{""},
		{"."},
	}
	if runtime.GOOS == "windows" {
		bad = append(bad, []string{`C:\Windows\evil.txt`}, []string{"C:evil"})
	}

	for _, parts := range bad {
		if _, err := cleanPath(parts); err == nil {
			t.Errorf("cleanPath(%q) did not reject an unsafe path", parts)
		}
	}
}

func TestSafeJoinStaysInsideRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "root")

	got, err := safeJoin(root, []string{"sub", "file.txt"})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "sub", "file.txt")
	if got != want {
		t.Errorf("safeJoin = %q, want %q", got, want)
	}

	if _, err := safeJoin(root, []string{"..", "escaped.txt"}); err == nil {
		t.Error("safeJoin did not reject a path outside the root")
	}
}
