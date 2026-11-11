package oidc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
)

type QrPicture struct{}

func (QrPicture) quietZone() int {
	return 4
}

func (QrPicture) moduleScale() int {
	return 8
}

func (QrPicture) white() int {
	return 255
}

func (QrPicture) prefix() string {
	return "data:image/png;base64,"
}

func (picture QrPicture) dataUri(matrix QrMatrix) (string, error) {
	var buffer bytes.Buffer
	err := png.Encode(&buffer, picture.draw(matrix))
	if err != nil {
		return "", err
	}
	return picture.prefix() + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}

func (picture QrPicture) draw(matrix QrMatrix) *image.Gray {
	padded := picture.padded(matrix)
	scale := picture.moduleScale()
	width := padded.size * scale
	result := image.NewGray(image.Rect(0, 0, width, width))
	for index := range result.Pix {
		cell := QrPosition{row: index / width / scale, column: index % width / scale}
		result.Pix[index] = uint8(picture.white() * (1 - padded.module(cell)))
	}
	return result
}

func (picture QrPicture) padded(matrix QrMatrix) QrMatrix {
	quiet := picture.quietZone()
	result := QrMatrix{}.sized(matrix.size + 2*quiet)
	for index, value := range matrix.modules {
		result.setData(QrPosition{row: index/matrix.size + quiet, column: index%matrix.size + quiet}, value)
	}
	return result
}
