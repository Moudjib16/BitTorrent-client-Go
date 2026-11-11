package main

import "testing"

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
