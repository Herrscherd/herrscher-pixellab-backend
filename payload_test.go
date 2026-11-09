package pixellab

import (
	"bytes"
	"encoding/json"
	"testing"
)

const payloadTestSeed int64 = 42

func payloadTestAccount() account {
	return account{secret: "s3cret", version: "v2", tier: 3}
}

func payloadFields(t *testing.T, j job) map[string]any {
	t.Helper()
	raw, err := json.Marshal(j.payload(payloadTestAccount(), payloadTestSeed))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func payloadRGBA() map[string]any {
	return map[string]any{"type": "rgba_bytes", "base64": "AAAA", "width": float64(128), "height": float64(128)}
}

func payloadImages(count int) []rgbaImage {
	images := make([]rgbaImage, 0, count)
	for range count {
		images = append(images, rgbaImage{base64: "AAAA", side: 128})
	}
	return images
}

func payloadAccountFields() map[string]any {
	return map[string]any{"version": "v2", "secret": "s3cret", "tier": float64(3)}
}

func assertPayload(t *testing.T, got, want map[string]any) {
	t.Helper()
	sameJSON(t, got, want)
}

func sameJSON(t *testing.T, got, want any) {
	t.Helper()
	gotJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	wantJSON, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("payload fields\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func merged(parts ...map[string]any) map[string]any {
	result := make(map[string]any)
	for _, part := range parts {
		for key, value := range part {
			result[key] = value
		}
	}
	return result
}

func TestEditPayloadMatchesTheProtocol(t *testing.T) {
	j := job{
		options: options{mode: modeEdit, description: "a knight", noBackground: true},
		size:    128,
		images:  payloadImages(1),
	}
	want := merged(payloadAccountFields(), map[string]any{
		"model_name":       "edit_image_pro",
		"description":      "a knight",
		"method":           "text",
		"seed":             "42",
		"no_background":    true,
		"use_selection":    false,
		"selection_origin": []any{float64(0), float64(0)},
		"output_method":    "New frame",
		"output_format":    "frames",
		"image_size":       map[string]any{"width": float64(128), "height": float64(128)},
		"edit_image":       payloadRGBA(),
	})
	assertPayload(t, payloadFields(t, j), want)
}

func TestInpaintPayloadMatchesTheProtocol(t *testing.T) {
	j := job{
		options: options{mode: modeInpaint, description: "a sword", noBackground: true},
		size:    128,
		images:  payloadImages(2),
	}
	want := merged(payloadAccountFields(), map[string]any{
		"model_name":           "generate_inpainting_v3",
		"description":          "a sword",
		"negative_description": "",
		"seed":                 "42",
		"no_background":        true,
		"crop_to_mask":         false,
		"use_selection":        false,
		"selection_origin":     []any{float64(0), float64(0)},
		"output_method":        "New layer",
		"output_format":        "frames",
		"image_size":           map[string]any{"width": float64(128), "height": float64(128)},
		"inpainting_image":     payloadRGBA(),
		"mask_image":           payloadRGBA(),
	})
	assertPayload(t, payloadFields(t, j), want)
}

func TestInpaintPayloadSendsTheNegativeAndTheCrop(t *testing.T) {
	j := job{
		options: options{mode: modeInpaint, description: "a sword", negative: "blurry", crop: true, noBackground: true},
		size:    128,
		images:  payloadImages(2),
	}
	fields := payloadFields(t, j)
	if fields["negative_description"] != "blurry" || fields["crop_to_mask"] != true {
		t.Fatalf("negative_description = %v, crop_to_mask = %v", fields["negative_description"], fields["crop_to_mask"])
	}
}

func TestAnimatePayloadMatchesTheProtocol(t *testing.T) {
	j := job{
		options: options{mode: modeAnimate, action: "walk", frames: defaultFrames, noBackground: true},
		size:    128,
		images:  payloadImages(1),
	}
	want := merged(payloadAccountFields(), map[string]any{
		"model_name":       "generate_animate_with_text_v3",
		"action":           "walk",
		"no_background":    true,
		"frame_count":      float64(defaultFrames),
		"seed":             "42",
		"use_selection":    false,
		"selection_origin": []any{float64(0), float64(0)},
		"output_method":    "New frame",
		"image_size":       map[string]any{"width": float64(128), "height": float64(128)},
		"first_frame":      payloadRGBA(),
	})
	assertPayload(t, payloadFields(t, j), want)
}

func TestAnimatePayloadAddsTheLastFrameWhenGiven(t *testing.T) {
	j := job{
		options: options{mode: modeAnimate, action: "walk", frames: defaultFrames, noBackground: true},
		size:    128,
		images:  payloadImages(2),
	}
	sameJSON(t, payloadFields(t, j)["last_frame"], payloadRGBA())
}

func TestNewPayloadMatchesTheProtocol(t *testing.T) {
	j := job{
		options: options{
			mode:         modeNew,
			description:  "a knight",
			noBackground: true,
			style:        styleOptions{outline: true, detail: true, shading: true},
		},
		size: 128,
	}
	want := merged(payloadAccountFields(), map[string]any{
		"model_name":           "generate_image_new",
		"description":          "a knight",
		"negative_description": "",
		"seed":                 float64(42),
		"no_background":        true,
		"output_method":        "New frame",
		"style_options": map[string]any{
			"color_palette": false,
			"outline":       true,
			"detail":        true,
			"shading":       true,
		},
		"image_size":  map[string]any{"width": float64(128), "height": float64(128)},
		"output_size": float64(128),
		"grid_rows":   float64(2),
		"grid_cols":   float64(2),
	})
	assertPayload(t, payloadFields(t, j), want)
}

func TestOnlyTheSelectionModesCarryTheSelection(t *testing.T) {
	newJob := job{options: options{mode: modeNew, noBackground: true}, size: 128}
	fields := payloadFields(t, newJob)
	for _, key := range []string{"use_selection", "selection_origin", "crop_to_mask"} {
		if _, found := fields[key]; found {
			t.Fatalf("new payload carries %q", key)
		}
	}
}
