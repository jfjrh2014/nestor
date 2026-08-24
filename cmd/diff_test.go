package cmd

import (
	"reflect"
	"sort"
	"testing"
)

func TestUntrackedPackages(t *testing.T) {
	configured := map[string]bool{
		"git":     true,
		"curl":    true,
		"tmux":    true,
		"jq":      true,
		"nonconf": false, // explicit — never in config
	}

	tests := []struct {
		name       string
		configured map[string]bool
		installed  []string
		want       []string
	}{
		{
			name:       "all installed are tracked",
			configured: configured,
			installed:  []string{"git", "curl"},
			want:       []string{},
		},
		{
			name:       "some installed are untracked",
			configured: configured,
			installed:  []string{"git", "ripgrep", "bat", "tmux"},
			want:       []string{"bat", "ripgrep"}, // sorted
		},
		{
			name:       "nothing installed",
			configured: configured,
			installed:  nil,
			want:       []string{},
		},
		{
			name:       "configured empty — everything is extra",
			configured: map[string]bool{},
			installed:  []string{"git", "vim"},
			want:       []string{"git", "vim"},
		},
		{
			name:       "dedups — installed list with repeats",
			configured: map[string]bool{"git": true},
			installed:  []string{"vim", "vim", "bat"},
			want:       []string{"bat", "vim"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sanity: confirm the expected slice is also sorted, since the
			// contract is "sorted for deterministic output".
			wantSorted := append([]string{}, tt.want...)
			sort.Strings(wantSorted)
			if !reflect.DeepEqual(wantSorted, tt.want) {
				t.Fatalf("test data: tt.want must be sorted; got %v", tt.want)
			}

			got := untrackedPackages(tt.configured, tt.installed)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("untrackedPackages() = %v, want %v", got, tt.want)
			}
		})
	}
}
