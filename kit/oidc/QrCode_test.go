package oidc

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func sampleUri(length int) string {
	head := "otpauth://totp/Mesail:"
	tail := "?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=Mesail&algorithm=SHA1&digits=6&period=30"
	label := "ana%40example.com"
	for len(head)+len(label)+len(tail) < length {
		label += "x"
	}
	text := head + label + tail
	if len(text) > length {
		text = text[:length]
	}
	return text
}

func samples() map[string]string {
	return map[string]string{
		"hello":   "hello",
		"uri-60":  "otpauth://totp/Mesail:ana?secret=JBSWY3DPEHPK3PXP&issuer=Mes",
		"uri-110": sampleUri(110),
		"uri-200": sampleUri(200),
		"uri-213": sampleUri(213),
	}
}

func decodePicture(t *testing.T, uri string) []byte {
	t.Helper()
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("missing prefix: %q", uri[:30])
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReedSolomonKnownVector(t *testing.T) {
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	got := QrPolynomial{}.remainder(data, 10)
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGeneratorPolynomialDegreeTen(t *testing.T) {
	want := []int{1, 216, 194, 159, 111, 199, 94, 95, 113, 157, 193}
	got := QrPolynomial{}.generator(10)
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestGaloisMultiply(t *testing.T) {
	if got := (QrGaloisField{}).multiply(2, 128); got != 0x1D {
		t.Fatalf("got %#x", got)
	}
	if got := (QrGaloisField{}).multiply(0, 77); got != 0 {
		t.Fatalf("got %#x", got)
	}
}

func TestFormatBitsForLevelM(t *testing.T) {
	want := []int{0x5412, 0x5125, 0x5E7C, 0x5B4B, 0x45F9, 0x40CE, 0x4F97, 0x4AA0}
	for mask, expected := range want {
		if got := (QrFormatInformation{}).formatBits(mask); got != expected {
			t.Fatalf("mask %d: got %#x want %#x", mask, got, expected)
		}
	}
}

func TestVersionBits(t *testing.T) {
	want := map[int]int{7: 0x07C94, 8: 0x085BC, 9: 0x09A99, 10: 0x0A4D3}
	for version, expected := range want {
		if got := (QrFormatInformation{}).versionBits(version); got != expected {
			t.Fatalf("version %d: got %#x want %#x", version, got, expected)
		}
	}
}

func TestDataEncoderHello(t *testing.T) {
	want := []byte{0x40, 0x56, 0x86, 0x56, 0xC6, 0xC6, 0xF0, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC, 0x11, 0xEC}
	got := QrDataEncoder{}.encode([]byte("hello"), 1)
	if !bytes.Equal(got, want) {
		t.Fatalf("got % X want % X", got, want)
	}
}

func TestVersionSelection(t *testing.T) {
	cases := map[int]int{0: 1, 14: 1, 15: 2, 62: 4, 63: 5, 106: 6, 107: 7, 180: 9, 181: 10, 213: 10}
	for length, expected := range cases {
		got, err := QrVersionChooser{}.choose(length)
		if err != nil || got != expected {
			t.Fatalf("length %d: got %d, %v want %d", length, got, err, expected)
		}
	}
}

func TestTooLong(t *testing.T) {
	_, err := QrCode{}.DataUri(strings.Repeat("a", 214))
	var tooLong QrTooLong
	if !errors.As(err, &tooLong) {
		t.Fatalf("got %v", err)
	}
	if tooLong.Length != 214 || tooLong.Capacity != 213 {
		t.Fatalf("got %+v", tooLong)
	}
	if !strings.Contains(err.Error(), "214") || !strings.Contains(err.Error(), "213") {
		t.Fatalf("message %q", err.Error())
	}
}

func TestBlockInterleaving(t *testing.T) {
	data := make([]byte, 154)
	for index := range data {
		data[index] = byte(index)
	}
	got := QrBlockInterleaver{}.codewords(data, 8)
	if len(got) != 242 {
		t.Fatalf("length %d", len(got))
	}
	if !bytes.Equal(got[:5], []byte{0, 38, 76, 115, 1}) {
		t.Fatalf("start % d", got[:5])
	}
	if !bytes.Equal(got[148:154], []byte{37, 75, 113, 152, 114, 153}) {
		t.Fatalf("end of data % d", got[148:154])
	}
}

func TestPenaltyParts(t *testing.T) {
	penalty := QrPenalty{}
	if got := penalty.lineRunScore([]int{1, 1, 1, 1, 1, 1, 0, 0, 0, 0}); got != 4 {
		t.Fatalf("run score %d", got)
	}
	if got := penalty.lineFinderScore(penalty.withQuietZone([]int{1, 0, 1, 1, 1, 0, 1})); got != 80 {
		t.Fatalf("finder score %d", got)
	}
	matrix := QrMatrix{}.sized(2)
	if got := penalty.blockScore(matrix); got != 3 {
		t.Fatalf("block score %d", got)
	}
	if got := penalty.balanceScore(matrix); got != 100 {
		t.Fatalf("balance score %d", got)
	}
}

func TestMaskSelectorPrefersLowerScore(t *testing.T) {
	low := QrCandidate{score: 1}
	high := QrCandidate{score: 2}
	if (QrMaskSelector{}).lower(high, low).score != 1 || (QrMaskSelector{}).lower(low, high).score != 1 {
		t.Fatal("lower picked the wrong candidate")
	}
}

func TestDataUriPictureSize(t *testing.T) {
	cases := map[string]int{"hello": 21, sampleUri(110): 45, sampleUri(200): 57}
	for text, size := range cases {
		uri, err := QrCode{}.DataUri(text)
		if err != nil {
			t.Fatal(err)
		}
		picture, err := png.Decode(bytes.NewReader(decodePicture(t, uri)))
		if err != nil {
			t.Fatal(err)
		}
		want := (size + 8) * 8
		bounds := picture.Bounds()
		if bounds.Dx() != want || bounds.Dy() != want {
			t.Fatalf("size %v want %d", bounds, want)
		}
	}
}

func TestSampleLengths(t *testing.T) {
	for name, text := range samples() {
		t.Logf("%s: %d bytes", name, len(text))
	}
	if len(samples()["uri-60"]) != 60 || len(samples()["uri-200"]) != 200 || len(samples()["uri-213"]) != 213 {
		t.Fatal("sample lengths are wrong")
	}
}

func TestWriteSamples(t *testing.T) {
	folder := os.Getenv("QR_SAMPLE_DIR")
	if folder == "" {
		t.Skip("QR_SAMPLE_DIR not set")
	}
	for name, text := range samples() {
		uri, err := QrCode{}.DataUri(text)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name+".png"), decodePicture(t, uri), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(folder, name+".txt"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
