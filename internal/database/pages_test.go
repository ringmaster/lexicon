package database

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Page Name", "page-name"},
		{"The Battle of Foo", "the-battle-of-foo"},
		{"Chapter 1", "chapter-1"},
		{"Some-Thing", "some-thing"},
		{"Hello_World", "hello-world"},
		{"  Spaces  ", "spaces"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"UPPERCASE", "uppercase"},
		{"MixedCase", "mixedcase"},
		{"Numbers123", "numbers123"},
		{"Special!@#$Characters", "specialcharacters"},
		{"---hyphens---", "hyphens"},
		{"", ""},
		{"Café", "cafe"},
		{"naïve", "naive"},
		{"Übermensch", "ubermensch"},
		{"日本語", ""},
		{"abc123", "abc123"},
		{"a-b-c", "a-b-c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
