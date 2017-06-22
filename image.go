/**
Eamon Collins
File for image manipulation
**/

package main

import (
  "bytes"
  "fmt"
  "image"
  "image/png"
  _"image/jpeg"
  "github.com/disintegration/imaging"
)

type ClarifaiBound struct {
  Top float32
  Bottom float32
  Left float32
  Right float32
}

func Clarifai_Image_Crop(bound ClarifaiBound, image_bytes []byte) {
  buf := bytes.NewBuffer(image_bytes)
  img, _, err := image.Decode(buf)
  if err != nil {
    fmt.Println(err)
  }
}

