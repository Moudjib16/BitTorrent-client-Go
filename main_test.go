package main

import (
	"testing"
	"time"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		size int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1024 * 1024, "1.0 MiB"},
		{3 * 1024 * 1024 * 1024, "3.0 GiB"},
	}

	for _, test := range tests {
		if got := humanSize(test.size); got != test.want {
			t.Errorf("humanSize(%d) = %q, want %q", test.size, got, test.want)
		}
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{-time.Second, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{time.Hour, "1h00m"},
		{2*time.Hour + 5*time.Minute, "2h05m"},
	}

	for _, test := range tests {
		if got := humanDuration(test.duration); got != test.want {
			t.Errorf("humanDuration(%v) = %q, want %q", test.duration, got, test.want)
		}
	}
}
