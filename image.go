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

type ClarifaiBound struct {
	Top, Bottom, Left, Right float32
}
type GoogleBound struct {
	Top, Bottom, Left, Right int
}

func Clarifai_Image_Crop(bound ClarifaiBound, image_bytes []byte) string {
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
	resized := imaging.Resize(cropped, 40, 40, imaging.Lanczos)

	var buff bytes.Buffer
	png.Encode(&buff, resized)
	b64string := base64.StdEncoding.EncodeToString(buff.Bytes())

	return b64string
}

//also works for microsoft with small adjustments in main.go's microsoft_request function
func Google_Image_Crop(bound GoogleBound, image_bytes []byte) string {
	buf := bytes.NewBuffer(image_bytes)
	img, _, err := image.Decode(buf)
	if err != nil {
		fmt.Println(err)
	}

	crop := image.Rectangle{image.Point{bound.Left, bound.Top}, image.Point{bound.Right, bound.Bottom}}
	cropped := imaging.Crop(img, crop)
	resized := imaging.Resize(cropped, 40, 40, imaging.Lanczos)

	var buff bytes.Buffer
	png.Encode(&buff, resized)
	b64string := base64.StdEncoding.EncodeToString(buff.Bytes())

	return b64string
}
