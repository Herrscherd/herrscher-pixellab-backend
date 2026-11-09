package pixellab

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

const (
	maxAttachmentBytes = 8 << 20
	maxFrameBytes      = 8 << 20
	rgbaChannels       = 4
	gridSmall          = 42
	gridMedium         = 85
	gridLarge          = 170
	gridDense          = 8
	gridMid            = 4
	gridSparse         = 2
)

func decodePNG(path string) (*image.NRGBA, error) {
	raw, err := readBounded(path, maxAttachmentBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrInvalidAttachment, path, err)
	}
	if !bytes.HasPrefix(raw, []byte(pngSignature)) {
		return nil, fmt.Errorf("%w: %s is not a PNG", ErrInvalidAttachment, path)
	}
	header, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: read the header of %s: %w", ErrInvalidAttachment, path, err)
	}
	if header.Width != header.Height || header.Width > maxSize {
		return nil, fmt.Errorf("%w: %s is %dx%d, want a square of at most %d px", ErrInvalidAttachment, path, header.Width, header.Height, maxSize)
	}
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: decode %s: %w", ErrInvalidAttachment, path, err)
	}
	bounds := decoded.Bounds()
	picture := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(picture, picture.Bounds(), decoded, bounds.Min, draw.Src)
	return picture, nil
}

func frameToPNG(encoded string, declared imageSize) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: image is not base64: %w", ErrServer, err)
	}
	if len(raw) > maxFrameBytes {
		return nil, fmt.Errorf("%w: image of %d bytes, at most %d allowed", ErrServer, len(raw), maxFrameBytes)
	}
	if bytes.HasPrefix(raw, []byte(pngSignature)) {
		return acceptPNG(raw)
	}
	size, err := frameDimensions(raw, declared)
	if err != nil {
		return nil, err
	}
	return encodePNG(raw, size.Width, size.Height)
}

func acceptPNG(raw []byte) ([]byte, error) {
	if err := checkPNG(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func checkPNG(raw []byte) error {
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%w: frame is not a valid PNG: %w", ErrServer, err)
	}
	return checkFrameSize(imageSize{Width: config.Width, Height: config.Height})
}

func checkFrameSize(size imageSize) error {
	if size.Width < 1 || size.Width > maxSize || size.Height < 1 || size.Height > maxSize {
		return fmt.Errorf("%w: frame of %dx%d px, want sides from 1 to %d", ErrServer, size.Width, size.Height, maxSize)
	}
	return nil
}

func frameDimensions(raw []byte, declared imageSize) (imageSize, error) {
	if declared.Width > 0 && declared.Height > 0 {
		if err := checkFrameSize(declared); err != nil {
			return imageSize{}, err
		}
		if declared.Width*declared.Height*rgbaChannels != len(raw) {
			return imageSize{}, fmt.Errorf("%w: %dx%d RGBA needs %d bytes, got %d", ErrServer, declared.Width, declared.Height, declared.Width*declared.Height*rgbaChannels, len(raw))
		}
		return declared, nil
	}
	pixels := len(raw) / rgbaChannels
	side := int(math.Sqrt(float64(pixels)))
	if side*side*rgbaChannels != len(raw) {
		return imageSize{}, fmt.Errorf("%w: %d bytes is not a square RGBA image", ErrServer, len(raw))
	}
	if err := checkFrameSize(imageSize{Width: side, Height: side}); err != nil {
		return imageSize{}, err
	}
	return imageSize{Width: side, Height: side}, nil
}

func encodePNG(raw []byte, width, height int) ([]byte, error) {
	if width*height*rgbaChannels != len(raw) {
		return nil, fmt.Errorf("%w: %dx%d RGBA needs %d bytes, got %d", ErrServer, width, height, width*height*rgbaChannels, len(raw))
	}
	picture := &image.NRGBA{
		Pix:    raw,
		Stride: width * rgbaChannels,
		Rect:   image.Rect(0, 0, width, height),
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	return buffer.Bytes(), nil
}

func gridFor(size int) int {
	switch {
	case size <= gridSmall:
		return gridDense
	case size <= gridMedium:
		return gridMid
	case size <= gridLarge:
		return gridSparse
	default:
		return 1
	}
}
