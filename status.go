package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"bittorrent-client-go/p2p"
)

// statusPrinter keeps one line on the screen and updates it in place.
type statusPrinter struct {
	mu        sync.Mutex
	lastLen   int
	lastTime  time.Time
	lastBytes int64
}

func newStatusPrinter() *statusPrinter {
	return &statusPrinter{lastTime: time.Now()}
}

func (s *statusPrinter) update(p p2p.Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var speed float64
	if elapsed := now.Sub(s.lastTime).Seconds(); elapsed > 0 {
		speed = float64(p.BytesDone-s.lastBytes) / elapsed
	}
	s.lastTime = now
	s.lastBytes = p.BytesDone

	line := fmt.Sprintf("  %6.2f%%  %s / %s  %s/s  peers %d/%d",
		p.Percent(),
		humanSize(p.BytesDone),
		humanSize(p.BytesTotal),
		humanSize(int64(speed)),
		p.Peers,
		p.PeersTotal)

	if speed > 1024 && p.BytesDone < p.BytesTotal {
		left := time.Duration(float64(p.BytesTotal-p.BytesDone)/speed) * time.Second
		line += fmt.Sprintf("  eta %s", humanDuration(left))
	}

	// Pad the line so a shorter one does not leave the old one behind.
	if len(line) < s.lastLen {
		line += strings.Repeat(" ", s.lastLen-len(line))
	}
	s.lastLen = len(line)

	fmt.Printf("\r%s", line)
}

// finish ends the status line, so the next output starts on a new line.
func (s *statusPrinter) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()

	fmt.Println()
	s.lastLen = 0
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}
