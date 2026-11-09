package pixellab

import (
	"encoding/base64"
	"fmt"
	"image"
)

type job struct {
	options

	size   int
	images []rgbaImage
}

type rgbaImage struct {
	base64 string
	side   int
}

type attachmentRange struct {
	least int
	most  int
}

func prepareJob(opts options, attachments []string) (job, error) {
	bounds := attachmentBounds(opts.mode)
	if len(attachments) < bounds.least || len(attachments) > bounds.most {
		return job{}, attachmentCountError(opts.mode, bounds, len(attachments))
	}
	decoded := make([]*image.NRGBA, 0, len(attachments))
	for _, path := range attachments {
		picture, err := decodePNG(path)
		if err != nil {
			return job{}, err
		}
		decoded = append(decoded, picture)
	}
	size, err := resolveSize(opts, decoded)
	if err != nil {
		return job{}, err
	}
	images := make([]rgbaImage, 0, len(decoded))
	for _, picture := range decoded {
		images = append(images, rgbaImage{base64: base64.StdEncoding.EncodeToString(picture.Pix), side: size})
	}
	return job{options: opts, size: size, images: images}, nil
}

const (
	singleAttachment = 1
	pairAttachments  = 2
)

func attachmentBounds(kind mode) attachmentRange {
	var bounds attachmentRange
	switch kind {
	case modeEdit:
		bounds = attachmentRange{least: singleAttachment, most: singleAttachment}
	case modeInpaint:
		bounds = attachmentRange{least: pairAttachments, most: pairAttachments}
	case modeAnimate:
		bounds = attachmentRange{least: singleAttachment, most: pairAttachments}
	case modeNew:
	}
	return bounds
}

func attachmentCountError(kind mode, bounds attachmentRange, got int) error {
	if kind == modeNew && got > 0 {
		return fmt.Errorf("%w: attachments need --mode edit, inpaint or animate", ErrInvalidAttachment)
	}
	return fmt.Errorf("%w: mode %s takes %d to %d attachments, got %d", ErrInvalidAttachment, kind, bounds.least, bounds.most, got)
}

func resolveSize(opts options, decoded []*image.NRGBA) (int, error) {
	if len(decoded) == 0 {
		return opts.size, nil
	}
	side := decoded[0].Bounds().Dx()
	for _, picture := range decoded {
		bounds := picture.Bounds()
		if bounds.Dx() != bounds.Dy() || bounds.Dx() != side {
			return 0, fmt.Errorf("%w: attachments must be squares of one side, got %dx%d", ErrInvalidAttachment, bounds.Dx(), bounds.Dy())
		}
	}
	if side > maxSize {
		return 0, fmt.Errorf("%w: attachments of %d px exceed the %d px limit", ErrInvalidAttachment, side, maxSize)
	}
	if opts.sizeGiven && opts.size != side {
		return 0, fmt.Errorf("%w: attachments are %d px, --size asks %d", ErrInvalidAttachment, side, opts.size)
	}
	return side, nil
}
