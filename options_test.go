package pixellab

import (
	"errors"
	"testing"
)

func TestParseOptions(t *testing.T) {
	defaults := settings{defaultSize: 64, maxImages: 4}
	cases := []struct {
		name         string
		content      string
		mode         mode
		description  string
		size         int
		count        int
		noBackground bool
		wantErr      bool
	}{
		{name: "plain prompt keeps defaults", content: "a red heart", mode: modeNew, description: "a red heart", size: 64, count: 1, noBackground: true},
		{name: "size and count", content: "--size 32 --n 2 knight", mode: modeNew, description: "knight", size: 32, count: 2, noBackground: true},
		{name: "equal sign form is not accepted as a value", content: "--size=32 knight", wantErr: true},
		{name: "quoted description keeps its spaces", content: `--mode new "a knight with a red cape"`, mode: modeNew, description: "a knight with a red cape", size: 64, count: 1, noBackground: true},
		{name: "background flag paints one", content: "--bg tree", mode: modeNew, description: "tree", size: 64, count: 1, noBackground: false},
		{name: "unknown flag", content: "--sparkle tree", wantErr: true},
		{name: "value flag without value", content: "tree --size", wantErr: true},
		{name: "size above the limit", content: "--size 300 tree", wantErr: true},
		{name: "count above max images", content: "--n 9 tree", wantErr: true},
		{name: "unknown mode", content: "--mode paint tree", wantErr: true},
		{name: "unclosed quote", content: `"a tree`, wantErr: true},
		{name: "missing description", content: "--size 32", wantErr: true},
		{name: "animate needs an action", content: "--mode animate", wantErr: true},
		{name: "animate with action", content: "--mode animate --action walk", mode: modeAnimate, size: 64, count: 1, noBackground: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseOptions(tc.content, defaults)
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidOption) {
					t.Fatalf("error = %v, want ErrInvalidOption", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseOptions(%q): %v", tc.content, err)
			}
			if opts.mode != tc.mode || opts.description != tc.description || opts.size != tc.size || opts.count != tc.count || opts.noBackground != tc.noBackground {
				t.Fatalf("options = %+v, want mode %s description %q size %d count %d bg %v", opts, tc.mode, tc.description, tc.size, tc.count, tc.noBackground)
			}
		})
	}
}

func TestTokenizeKeepsQuotedSpaces(t *testing.T) {
	tokens, err := tokenize(`a "b c" d`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b c", "d"}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %q, want %q", tokens, want)
	}
	for index := range want {
		if tokens[index] != want[index] {
			t.Fatalf("tokens = %q, want %q", tokens, want)
		}
	}
}

func TestStyleFlagsTurnOffOneToggle(t *testing.T) {
	opts, err := parseOptions("--no-outline --palette tree", settings{defaultSize: 64, maxImages: 4})
	if err != nil {
		t.Fatal(err)
	}
	if opts.style.outline || !opts.style.detail || !opts.style.shading || !opts.style.palette {
		t.Fatalf("style = %+v, want outline off and palette on", opts.style)
	}
}

func TestSeedFlagIsPinned(t *testing.T) {
	opts, err := parseOptions("--seed 42 tree", settings{defaultSize: 64, maxImages: 4})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.seedGiven || opts.seed != 42 {
		t.Fatalf("seed = %d given %v, want 42", opts.seed, opts.seedGiven)
	}
}
