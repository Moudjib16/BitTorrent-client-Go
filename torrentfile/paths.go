package torrentfile

import (
	"fmt"
	"path/filepath"
	"strings"
)

// cleanPath turns the path components from a torrent into a path we are
// willing to create. A torrent is just a file from the internet, so its paths
// can contain things like ".." and try to escape the download directory.
func cleanPath(parts []string) ([]string, error) {
	var cleaned []string
	for _, part := range parts {
		// Some torrents put more than one component in a single string.
		for _, piece := range strings.FieldsFunc(part, func(r rune) bool {
			return r == '/' || r == '\\'
		}) {
			if piece == "." || piece == ".." {
				return nil, fmt.Errorf("torrent file path %q is not safe", strings.Join(parts, "/"))
			}
			if filepath.IsAbs(piece) || filepath.VolumeName(piece) != "" {
				return nil, fmt.Errorf("torrent file path %q is not safe", strings.Join(parts, "/"))
			}
			cleaned = append(cleaned, piece)
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("torrent contains an empty file path")
	}
	return cleaned, nil
}

// safeJoin joins a torrent file path onto root and makes sure the result is
// still inside root.
func safeJoin(root string, parts []string) (string, error) {
	cleaned, err := cleanPath(parts)
	if err != nil {
		return "", err
	}

	full := filepath.Join(append([]string{root}, cleaned...)...)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("torrent file path %q is not safe", strings.Join(parts, "/"))
	}
	return full, nil
}
