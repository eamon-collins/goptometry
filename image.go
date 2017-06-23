/**
Eamon Collins
File for image manipulation
**/

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	_ "image/jpeg"
	"image/png"
)

//size of each dimension of the thumbnail for logos and
//faces displayed in the results panes
const thumbnailSize int = 60

//accepts floats as ratios of the total image dimensions
type RatioBound struct {
	Top, Bottom, Left, Right float32
}

//accepts ints as actual pixel crop specifications
type PixelBound struct {
	Top, Bottom, Left, Right int
}

//Crop specified by floats representing ratios of the total image dimension
func Ratio_Image_Crop(bound RatioBound, image_bytes []byte) string {
	//decode bytes into image.Image instance
	buf := bytes.NewBuffer(image_bytes)
	img, _, err := image.Decode(buf)
	if err != nil {
		fmt.Println(err)
	}

	//Clarifai's bounds expressed as a percentage, so have to multiply by dimensions
	//to get actual pixel values to crop at
	mX := img.Bounds().Max.X
	mY := img.Bounds().Max.Y
	crop := image.Rectangle{image.Point{int(bound.Left * float32(mX)), int(bound.Top * float32(mY))}, image.Point{int(bound.Right * float32(mX)), int(bound.Bottom * float32(mY))}}
	cropped := imaging.Crop(img, crop)
	resized := imaging.Resize(cropped, thumbnailSize, thumbnailSize, imaging.Lanczos)

	var buff bytes.Buffer
	png.Encode(&buff, resized)
	b64string := base64.StdEncoding.EncodeToString(buff.Bytes())

	return b64string
}

//Crop specified by actual pixel values
func Pixel_Image_Crop(bound PixelBound, image_bytes []byte) string {
	buf := bytes.NewBuffer(image_bytes)
	img, _, err := image.Decode(buf)
	if err != nil {
		fmt.Println(err)
	}

	crop := image.Rectangle{image.Point{bound.Left, bound.Top}, image.Point{bound.Right, bound.Bottom}}
	cropped := imaging.Crop(img, crop)
	resized := imaging.Resize(cropped, thumbnailSize, thumbnailSize, imaging.Lanczos)

	var buff bytes.Buffer
	png.Encode(&buff, resized)
	b64string := base64.StdEncoding.EncodeToString(buff.Bytes())

	return b64string
}

func Resize_Initial_Image(image_bytes []byte) string {
	buf := bytes.NewBuffer(image_bytes)
	img, _, err := image.Decode(buf)
	if err != nil {
		fmt.Println(err)
	}

	resized := imaging.Resize(img, 400, 0, imaging.Lanczos)

	var buff bytes.Buffer
	png.Encode(&buff, resized)
	b64string := base64.StdEncoding.EncodeToString(buff.Bytes())

	return b64string
}
