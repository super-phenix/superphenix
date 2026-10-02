package names

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

const maxNameLength = 63

func TestWordLists(t *testing.T) {
	wordRegex := regexp.MustCompile(`^[a-z]+$`)

	tests := []struct {
		name  string
		words []string
	}{
		{name: "adjectives", words: adjectives},
		{name: "colors", words: colors},
		{name: "animals", words: animals},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.words) == 0 {
				t.Fatal("word list is empty")
			}
			seen := make(map[string]bool, len(tt.words))
			for _, word := range tt.words {
				if !wordRegex.MatchString(word) {
					t.Errorf("invalid word %q", word)
				}
				if seen[word] {
					t.Errorf("duplicate word %q", word)
				}
				seen[word] = true
			}
		})
	}
}

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		position int
		words    []string
	}{
		{name: "first word is an adjective", position: 0, words: adjectives},
		{name: "second word is a color", position: 1, words: colors},
		{name: "third word is an animal", position: 2, words: animals},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 200 {
				name := Generate()
				if len(name) > maxNameLength {
					t.Fatalf("name %q is longer than %d characters", name, maxNameLength)
				}
				parts := strings.Split(name, "-")
				if len(parts) != 3 {
					t.Fatalf("name %q does not have 3 words", name)
				}
				if !slices.Contains(tt.words, parts[tt.position]) {
					t.Fatalf("unexpected word %q in name %q", parts[tt.position], name)
				}
			}
		})
	}
}
