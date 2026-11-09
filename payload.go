package pixellab

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"
)

const (
	methodText     = "text"
	outputNewFrame = "New frame"
	outputNewLayer = "New layer"
	outputFrames   = "frames"
	modelNew       = "generate_image_new"
	modelEdit      = "edit_image_pro"
	modelInpaint   = "generate_inpainting_v3"
	modelAnimate   = "generate_animate_with_text_v3"
	pathNew        = "/generate-image-new"
	pathEdit       = "/generate-edit-image-pro"
	pathInpaint    = "/generate-inpainting-v3"
	pathAnimate    = "/animate-with-text-v3"
)

type imageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type rgbaField struct {
	Type   string `json:"type"`
	Base64 string `json:"base64"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type styleFields struct {
	ColorPalette bool `json:"color_palette"`
	Outline      bool `json:"outline"`
	Detail       bool `json:"detail"`
	Shading      bool `json:"shading"`
}

type selectionFields struct {
	UseSelection    bool   `json:"use_selection"`
	SelectionOrigin [2]int `json:"selection_origin"`
}

func noSelection() *selectionFields {
	return &selectionFields{}
}

type payload struct {
	*selectionFields

	ModelName           string       `json:"model_name"`
	Description         string       `json:"description,omitempty"`
	NegativeDescription *string      `json:"negative_description,omitempty"`
	Action              string       `json:"action,omitempty"`
	Method              string       `json:"method,omitempty"`
	Seed                any          `json:"seed"`
	NoBackground        bool         `json:"no_background"`
	CropToMask          *bool        `json:"crop_to_mask,omitempty"`
	OutputMethod        string       `json:"output_method"`
	OutputFormat        string       `json:"output_format,omitempty"`
	FrameCount          int          `json:"frame_count,omitempty"`
	ImageSize           imageSize    `json:"image_size"`
	StyleOptions        *styleFields `json:"style_options,omitempty"`
	OutputSize          int          `json:"output_size,omitempty"`
	GridRows            int          `json:"grid_rows,omitempty"`
	GridCols            int          `json:"grid_cols,omitempty"`
	EditImage           *rgbaField   `json:"edit_image,omitempty"`
	InpaintingImage     *rgbaField   `json:"inpainting_image,omitempty"`
	MaskImage           *rgbaField   `json:"mask_image,omitempty"`
	FirstFrame          *rgbaField   `json:"first_frame,omitempty"`
	LastFrame           *rgbaField   `json:"last_frame,omitempty"`
	Version             string       `json:"version"`
	Secret              string       `json:"secret"`
	Tier                int          `json:"tier"`
}

func (j job) path() string {
	switch j.mode {
	case modeEdit:
		return pathEdit
	case modeInpaint:
		return pathInpaint
	case modeAnimate:
		return pathAnimate
	case modeNew:
	}
	return pathNew
}

func (j job) payload(acc account, seed int64) payload {
	size := imageSize{Width: j.size, Height: j.size}
	base := payload{
		Version:      acc.version,
		Secret:       acc.secret,
		Tier:         acc.tier,
		ImageSize:    size,
		NoBackground: j.noBackground,
	}
	switch j.mode {
	case modeEdit:
		base.selectionFields = noSelection()
		base.ModelName = modelEdit
		base.Description = j.description
		base.Method = methodText
		base.Seed = strconv.FormatInt(seed, 10)
		base.OutputMethod = outputNewFrame
		base.OutputFormat = outputFrames
		base.EditImage = j.field(0)
	case modeInpaint:
		base.selectionFields = noSelection()
		base.ModelName = modelInpaint
		base.Description = j.description
		base.NegativeDescription = stringPointer(j.negative)
		base.Seed = strconv.FormatInt(seed, 10)
		base.CropToMask = boolPointer(j.crop)
		base.OutputMethod = outputNewLayer
		base.OutputFormat = outputFrames
		base.InpaintingImage = j.field(0)
		base.MaskImage = j.field(1)
	case modeAnimate:
		base.selectionFields = noSelection()
		base.ModelName = modelAnimate
		base.Action = j.action
		base.Seed = strconv.FormatInt(seed, 10)
		base.FrameCount = j.frames
		base.OutputMethod = outputNewFrame
		base.FirstFrame = j.field(0)
		base.LastFrame = j.field(1)
	case modeNew:
		grid := j.grid
		if grid == 0 {
			grid = gridFor(j.size)
		}
		style := styleFields{ColorPalette: j.style.palette, Outline: j.style.outline, Detail: j.style.detail, Shading: j.style.shading}
		base.ModelName = modelNew
		base.Description = j.description
		base.NegativeDescription = stringPointer(j.negative)
		base.Seed = seed
		base.OutputMethod = outputNewFrame
		base.StyleOptions = &style
		base.OutputSize = j.size
		base.GridRows = grid
		base.GridCols = grid
	}
	return base
}

func (j job) field(index int) *rgbaField {
	if index >= len(j.images) {
		return nil
	}
	image := j.images[index]
	return &rgbaField{Type: "rgba_bytes", Base64: image.base64, Width: image.side, Height: image.side}
}

func stringPointer(value string) *string {
	return &value
}

func boolPointer(value bool) *bool {
	return &value
}

func randomSeed() (int64, error) {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return 0, fmt.Errorf("read seed: %w", err)
	}
	return int64(binary.LittleEndian.Uint64(buffer[:])>>1) % (maxSeed + 1), nil
}
