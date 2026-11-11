package p2p

import "testing"

func TestProgressPercent(t *testing.T) {
	tests := []struct {
		progress Progress
		want     float64
	}{
		{Progress{BytesDone: 0, BytesTotal: 100}, 0},
		{Progress{BytesDone: 25, BytesTotal: 100}, 25},
		{Progress{BytesDone: 100, BytesTotal: 100}, 100},
		{Progress{BytesDone: 50, BytesTotal: 0}, 0},
	}

	for _, test := range tests {
		if got := test.progress.Percent(); got != test.want {
			t.Errorf("Percent() = %v, want %v", got, test.want)
		}
	}
}
