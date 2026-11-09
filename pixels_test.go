package pixellab

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestEncodePNGKeepsStraightAlpha(t *testing.T) {
	raw := []byte{255, 0, 0, 255, 0, 255, 0, 128}
	data, err := encodePNG(raw, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	red := decoded.At(0, 0)
	r, g, b, a := red.RGBA()
	if r>>8 != 255 || g != 0 || b != 0 || a>>8 != 255 {
		t.Fatalf("first pixel = %v, want opaque red", red)
	}
	picture, ok := decoded.(*image.NRGBA)
	if !ok {
		t.Fatalf("decoded %T, want *image.NRGBA", decoded)
	}
	straight := picture.NRGBAAt(1, 0)
	if straight.G != 255 || straight.A != 128 {
		t.Fatalf("second pixel = %v, want green at alpha 128", straight)
	}
}

func TestEncodePNGRejectsAShortBuffer(t *testing.T) {
	if _, err := encodePNG([]byte{1, 2, 3}, 1, 1); !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer", err)
	}
}

func TestEncodePNGPassesAPNGThrough(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	data, err := encodePNG(buffer.Bytes(), 1, 1)
	if err != nil || !bytes.Equal(data, buffer.Bytes()) {
		t.Fatalf("encodePNG did not return the PNG unchanged: %v", err)
	}
}

func TestFrameDimensionsInfersASquare(t *testing.T) {
	size, err := frameDimensions(make([]byte, 4*16), imageSize{})
	if err != nil || size.Width != 4 || size.Height != 4 {
		t.Fatalf("dimensions = %+v, %v, want 4x4", size, err)
	}
	if _, err := frameDimensions(make([]byte, 4*5), imageSize{}); !errors.Is(err, ErrServer) {
		t.Fatalf("error = %v, want ErrServer for a non square", err)
	}
}

func TestGridForSize(t *testing.T) {
	cases := map[int]int{8: 8, 42: 8, 43: 4, 85: 4, 86: 2, 170: 2, 171: 1, 256: 1}
	for size, want := range cases {
		if got := gridFor(size); got != want {
			t.Errorf("gridFor(%d) = %d, want %d", size, got, want)
		}
	}
}

func TestDecodePNGReadsAttachmentsAsRGBA(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	picture.Pix[0] = 200
	picture.Pix[3] = 255
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, picture); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "in.png")
	if err := os.WriteFile(path, buffer.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	decoded, err := decodePNG(path)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() != 2 || len(decoded.Pix) != 16 || decoded.Pix[0] != 200 {
		t.Fatalf("decoded %v with %d bytes", decoded.Bounds(), len(decoded.Pix))
	}
	if base64.StdEncoding.EncodeToString(decoded.Pix) == "" {
		t.Fatal("empty base64 for the attachment")
	}
}

func TestDecodePNGRefusesOtherFormats(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.jpg")
	if err := os.WriteFile(path, []byte("\xff\xd8\xff not a png"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := decodePNG(path); !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("error = %v, want ErrInvalidAttachment", err)
	}
}

func TestDecodePNGRefusesAHeaderLargerThanTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.png")
	if err := os.WriteFile(path, headerOnlyPNG(40000, 40000), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := decodePNG(path); !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("error = %v, want ErrInvalidAttachment for a 40000 px header", err)
	}
}

func TestDecodePNGRefusesAFileLargerThanTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(pngSignature); err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxAttachmentBytes + 1); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := decodePNG(path); !errors.Is(err, ErrInvalidAttachment) {
		t.Fatalf("error = %v, want ErrInvalidAttachment for a file over the limit", err)
	}
}

func headerOnlyPNG(width, height uint32) []byte {
	header := binary.BigEndian.AppendUint32(nil, width)
	header = binary.BigEndian.AppendUint32(header, height)
	header = append(header, 8, 6, 0, 0, 0)
	chunk := append([]byte("IHDR"), header...)
	data := []byte(pngSignature)
	data = binary.BigEndian.AppendUint32(data, uint32(len(header)))
	data = append(data, chunk...)
	return binary.BigEndian.AppendUint32(data, crc32.ChecksumIEEE(chunk))
}
