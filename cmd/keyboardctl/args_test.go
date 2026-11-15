package main

import "testing"

func TestResolveKeyArg(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		args  []string
		other []string
		want  string
	}{
		{"bare argument becomes the key", "", []string{"a"}, nil, "a"},
		{"bare hex argument", "", []string{"0x1E"}, nil, "0x1E"},
		{"explicit -key wins", "b", []string{"a"}, nil, "b"},
		{"no argument leaves the key alone", "", nil, nil, ""},
		{"two arguments are not a key", "", []string{"a", "b"}, nil, ""},
		{"other action flag blocks the shorthand", "", []string{"a"}, []string{"", "", "", "text"}, ""},
		{"empty other flags still allow the shorthand", "", []string{"a"}, []string{"", "", ""}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveKeyArg(tt.key, tt.args, tt.other...); got != tt.want {
				t.Fatalf("resolveKeyArg(%q, %v, %v) = %q, want %q", tt.key, tt.args, tt.other, got, tt.want)
			}
		})
	}
}

func TestCountNonEmpty(t *testing.T) {
	if got := countNonEmpty("", "", ""); got != 0 {
		t.Fatalf("all empty = %d, want 0", got)
	}
	if got := countNonEmpty("a", "", "c"); got != 2 {
		t.Fatalf("mixed = %d, want 2", got)
	}
}
