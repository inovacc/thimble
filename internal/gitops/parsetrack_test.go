package gitops

import "testing"

func TestParseTrackInfo(t *testing.T) {
	tests := []struct {
		name       string
		track      string
		wantAhead  int
		wantBehind int
	}{
		{"ahead only", "[ahead 3]", 3, 0},
		{"behind only", "[behind 5]", 0, 5},
		{"ahead and behind", "[ahead 2, behind 7]", 2, 7},
		{"no tracking", "", 0, 0},
		{"gone", "[gone]", 0, 0},
		{"large numbers", "[ahead 100, behind 200]", 100, 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b Branch
			parseTrackInfo(tt.track, &b)

			if b.Ahead != tt.wantAhead {
				t.Errorf("Ahead = %d, want %d", b.Ahead, tt.wantAhead)
			}

			if b.Behind != tt.wantBehind {
				t.Errorf("Behind = %d, want %d", b.Behind, tt.wantBehind)
			}
		})
	}
}
