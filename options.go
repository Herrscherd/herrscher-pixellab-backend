package pixellab

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type mode string

const maxRequests = 1024

const (
	modeNew     mode = "new"
	modeEdit    mode = "edit"
	modeInpaint mode = "inpaint"
	modeAnimate mode = "animate"
)

type styleOptions struct {
	palette bool
	outline bool
	detail  bool
	shading bool
}

type options struct {
	mode         mode
	description  string
	negative     string
	size         int
	sizeGiven    bool
	count        int
	seed         int64
	seedGiven    bool
	grid         int
	noBackground bool
	crop         bool
	action       string
	frames       int
	style        styleOptions
}

func booleanFlags() map[string]func(*options) {
	return map[string]func(*options){
		"bg":         func(opts *options) { opts.noBackground = false },
		"palette":    func(opts *options) { opts.style.palette = true },
		"no-outline": func(opts *options) { opts.style.outline = false },
		"no-detail":  func(opts *options) { opts.style.detail = false },
		"no-shading": func(opts *options) { opts.style.shading = false },
		"crop":       func(opts *options) { opts.crop = true },
	}
}

func valueFlags() map[string]func(*options, string) error {
	return map[string]func(*options, string) error{
		"size":   setSize,
		"n":      setCount,
		"seed":   setSeed,
		"grid":   setGrid,
		"neg":    func(opts *options, value string) error { opts.negative = value; return nil },
		"mode":   setMode,
		"action": func(opts *options, value string) error { opts.action = value; return nil },
		"frames": setFrames,
	}
}

func parseOptions(content string, cfg settings) (options, error) {
	tokens, err := tokenize(content)
	if err != nil {
		return options{}, err
	}
	parsed := options{
		mode:         modeNew,
		size:         cfg.defaultSize,
		count:        1,
		noBackground: true,
		frames:       defaultFrames,
		style:        styleOptions{outline: true, detail: true, shading: true},
	}
	var words []string
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		name, ok := strings.CutPrefix(token, "--")
		if !ok {
			words = append(words, token)
			continue
		}
		if apply, found := booleanFlags()[name]; found {
			apply(&parsed)
			continue
		}
		apply, found := valueFlags()[name]
		if !found {
			return options{}, fmt.Errorf("%w: unknown flag --%s", ErrInvalidOption, name)
		}
		if index+1 >= len(tokens) {
			return options{}, fmt.Errorf("%w: flag --%s needs a value", ErrInvalidOption, name)
		}
		index++
		if err := apply(&parsed, tokens[index]); err != nil {
			return options{}, err
		}
	}
	parsed.description = strings.Join(words, " ")
	return parsed, parsed.validate(cfg)
}

func (opts options) validate(cfg settings) error {
	if opts.count < 1 || opts.count > cfg.maxImages {
		return fmt.Errorf("%w: --n must be between 1 and %d", ErrInvalidOption, cfg.maxImages)
	}
	if opts.mode == modeAnimate && opts.action == "" {
		return fmt.Errorf("%w: animate needs --action", ErrInvalidOption)
	}
	if opts.mode != modeAnimate && opts.description == "" {
		return fmt.Errorf("%w: the prompt needs a description", ErrInvalidOption)
	}
	return nil
}

func tokenize(text string) ([]string, error) {
	var tokens []string
	var current []rune
	inQuotes, inToken := false, false
	for _, char := range text {
		switch {
		case char == '"':
			inQuotes = !inQuotes
			inToken = true
		case unicode.IsSpace(char) && !inQuotes:
			if inToken {
				tokens = append(tokens, string(current))
				current = current[:0]
				inToken = false
			}
		default:
			current = append(current, char)
			inToken = true
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("%w: unclosed quote", ErrInvalidOption)
	}
	if inToken {
		tokens = append(tokens, string(current))
	}
	return tokens, nil
}

func setSize(opts *options, value string) error {
	size, err := boundedInt("size", value, maxSize)
	if err != nil {
		return err
	}
	opts.size = size
	opts.sizeGiven = true
	return nil
}

func setCount(opts *options, value string) error {
	count, err := boundedInt("n", value, maxRequests)
	if err != nil {
		return err
	}
	opts.count = count
	return nil
}

func setGrid(opts *options, value string) error {
	grid, err := boundedInt("grid", value, maxGrid)
	if err != nil {
		return err
	}
	opts.grid = grid
	return nil
}

func setFrames(opts *options, value string) error {
	frames, err := boundedInt("frames", value, maxFrames)
	if err != nil {
		return err
	}
	opts.frames = frames
	return nil
}

func setSeed(opts *options, value string) error {
	seed, err := strconv.ParseInt(value, 10, 63)
	if err != nil || seed < 0 {
		return fmt.Errorf("%w: --seed %q is not a positive number", ErrInvalidOption, value)
	}
	opts.seed = seed
	opts.seedGiven = true
	return nil
}

func setMode(opts *options, value string) error {
	switch parsed := mode(value); parsed {
	case modeNew, modeEdit, modeInpaint, modeAnimate:
		opts.mode = parsed
		return nil
	}
	return fmt.Errorf("%w: --mode %q, want new, edit, inpaint or animate", ErrInvalidOption, value)
}

func boundedInt(name, value string, high int) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > high {
		return 0, fmt.Errorf("%w: --%s %q, want a number from 1 to %d", ErrInvalidOption, name, value, high)
	}
	return number, nil
}
