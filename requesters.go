/**
Eamon Collins
File for constructing requests
**/

package main

import (
  "bytes"
  "fmt"
  "net/http"
  "time"
  "io/ioutil"
  "encoding/json"
  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/rekognition"
  "github.com/eamon-collins/goptometry/secrets"
)

func request_clarifai(imgurl string, image_bytes []byte, model_id string) *Company {
  //response structure
  type ClarifaiJson struct {
    Outputs []struct {
      Data struct {
        Concepts []struct {
          Label string  `json:"name"`
          Score float32 `json:"value"`
        } `json:"concepts"`
        Regions []struct {
          Data struct {
            Face struct {
              Identity struct {
                Concepts []struct {
                  Label string  `json:"name"`
                  Score float32 `json:"value"`
                } `json:"concepts"`
              } `json:"identity"`
            } `json:"face"`
            Concepts []struct {
              Label string  `json:"name"`
              Score float32 `json:"value"`
            } `json:"concepts"`
          } `json:"data"`
          Region_Info struct {
            Bounding_Box struct {
              Top    float32 `json:"top_row"`
              Left   float32 `json:"left_col"`
              Bottom float32 `json:"bottom_row"`
              Right  float32 `json:"right_col"`
            } `json:"bounding_box"`
          } `json:"region_info"`
        } `json:"regions"`
      } `json:"data"`
    } `json:"outputs"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10,
  }
  body := []byte(`{"inputs": [ { "data": {"image": { "url":"` + imgurl + `"}}}]}`)
  req, err := http.NewRequest("POST", "https://api.clarifai.com/v2/models/"+model_id+"/outputs", bytes.NewBuffer(body))

  req.Header.Set("Authorization", "Bearer "+secrets.Clarifai_Token)
  req.Header.Set("Content-Type", "application/json")

  //make the request
  start := time.Now()
  resp, err := client.Do(req)
  elapsed := time.Since(start)
  if err != nil {
    panic(err)
  }

  //parse the response
  defer resp.Body.Close()
  data, _ := ioutil.ReadAll(resp.Body)
  fmt.Println(string(data))
  var dat ClarifaiJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }

  c := Company{Company: "Clarifai", Elapsed: elapsed.Seconds()}
  if model_id == "e466caa0619f444ab97497640cefc4dc" { //Celebrity
    //take the top result from each region, ie the most probable
    //identity for each face detected
    for _, celeb := range dat.Outputs[0].Data.Regions {
      c.Tags = append(c.Tags, Tag{Label: celeb.Data.Face.Identity.Concepts[0].Label, Score: celeb.Data.Face.Identity.Concepts[0].Score})
    }
  } else if model_id == "c443119bf2ed4da98487520d01a0b1e3" { //Logo
    for _, logo := range dat.Outputs[0].Data.Regions {
      box := RatioBound{Top: logo.Region_Info.Bounding_Box.Top,
        Bottom: logo.Region_Info.Bounding_Box.Bottom,
        Left:   logo.Region_Info.Bounding_Box.Left,
        Right:  logo.Region_Info.Bounding_Box.Right}
      c.Tags = append(c.Tags, Tag{Label: logo.Data.Concepts[0].Label, Image: Ratio_Image_Crop(box, image_bytes), Score: logo.Data.Concepts[0].Score})
    }
  } else if model_id == "a403429f2ddf4b49b307e318f00e528b" { //Face detection
    for _, face := range dat.Outputs[0].Data.Regions {
      box := RatioBound{Top: face.Region_Info.Bounding_Box.Top,
        Bottom: face.Region_Info.Bounding_Box.Bottom,
        Left:   face.Region_Info.Bounding_Box.Left,
        Right:  face.Region_Info.Bounding_Box.Right}
      fmt.Println(box)
      c.Tags = append(c.Tags, Tag{Image: Ratio_Image_Crop(box, image_bytes)})
    }
  } else { //General and everything that follow that format
    for _, tag := range dat.Outputs[0].Data.Concepts {
      c.Tags = append(c.Tags, Tag{Label: tag.Label, Score: tag.Score})
    }
  }
  return &c
}

func request_google(imgurl string, image_bytes []byte, model_id string) *Company {
  //response structure
  type GoogleJson struct {
    Responses []struct {
      LabelAnnotations []struct {
        Label string  `json:"description"`
        Score float32 `json:"score"`
      } `json:"labelAnnotations"`
      LogoAnnotations []struct {
        Label        string  `json:"description"`
        Score        float32 `json:"score"`
        BoundingPoly struct {
          Vertices []struct {
            X int `json:"x"`
            Y int `json:"y"`
          } `json:"vertices"`
        } `json:"boundingPoly"`
      } `json:"LogoAnnotations"` //come back and deal with bounding box?
      SafeSearchAnnotation struct {
        Adult    string `json:"adult"`
        Spoof    string `json:"spoof"`
        Medical  string `json:"medical"`
        Violence string `json:"violence"`
      } `json:"safeSearchAnnotation"`
      FaceAnnotations []struct {
        BoundingPoly struct {
          Vertices []struct {
            X int `json:"x"`
            Y int `json:"y"`
          } `json:"vertices"`
        } `json:"boundingPoly"`
        Score float32 `json:"detectionConfidence"`
      } `json:"faceAnnotations"`
    } `json:"responses"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10,
  }
  //vary based on type of request to be made
  var request_type string
  if model_id == "General" {
    request_type = "LABEL_DETECTION"
  } else if model_id == "NSFW" {
    request_type = "SAFE_SEARCH_DETECTION"
  } else if model_id == "Logo" {
    request_type = "LOGO_DETECTION"
  } else if model_id == "Face" {
    request_type = "FACE_DETECTION"
  }

  body := []byte(`{requests:[{
    "image":{
      "source":{
        "imageUri":"` + imgurl + `"}},
        "features":[{
          "type":"` + request_type + `",
          "maxResults":20}]}]}`)
  req, err := http.NewRequest("POST", "https://vision.googleapis.com/v1/images:annotate", bytes.NewBuffer(body))
  q := req.URL.Query()
  q.Add("key", secrets.Google_Api_Key)
  req.URL.RawQuery = q.Encode()

  //make the request
  start := time.Now()
  resp, err := client.Do(req)
  elapsed := time.Since(start)
  if err != nil {
    panic(err)
  }

  //parse the response
  defer resp.Body.Close()
  data, _ := ioutil.ReadAll(resp.Body)
  fmt.Println(string(data))
  var dat GoogleJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }

  g := Company{Company: "Google", Elapsed: elapsed.Seconds()}
  if model_id == "General" {
    for _, tag := range dat.Responses[0].LabelAnnotations {
      g.Tags = append(g.Tags, Tag{Label: tag.Label, Score: tag.Score})
    }
  } else if model_id == "NSFW" {
    g.Tags = append(g.Tags, Tag{Label: "adult", Description: dat.Responses[0].SafeSearchAnnotation.Adult})
    g.Tags = append(g.Tags, Tag{Label: "spoof", Description: dat.Responses[0].SafeSearchAnnotation.Spoof})
    g.Tags = append(g.Tags, Tag{Label: "medical", Description: dat.Responses[0].SafeSearchAnnotation.Medical})
    g.Tags = append(g.Tags, Tag{Label: "violence", Description: dat.Responses[0].SafeSearchAnnotation.Violence})
  } else if model_id == "Logo" {
    for _, tag := range dat.Responses[0].LogoAnnotations {
      box := PixelBound{Top: tag.BoundingPoly.Vertices[0].Y,
        Bottom: tag.BoundingPoly.Vertices[2].Y,
        Left:   tag.BoundingPoly.Vertices[0].X,
        Right:  tag.BoundingPoly.Vertices[2].X}
      g.Tags = append(g.Tags, Tag{Label: tag.Label, Score: tag.Score, Image: Pixel_Image_Crop(box, image_bytes)})
    }
  } else if model_id == "Face" {
    for _, tag := range dat.Responses[0].FaceAnnotations {
      box := PixelBound{Top: tag.BoundingPoly.Vertices[0].Y,
        Bottom: tag.BoundingPoly.Vertices[2].Y,
        Left:   tag.BoundingPoly.Vertices[0].X,
        Right:  tag.BoundingPoly.Vertices[2].X}
      g.Tags = append(g.Tags, Tag{Image: Pixel_Image_Crop(box, image_bytes), Score: tag.Score})
    }
  }
  return &g
}

func request_microsoft(imgurl string, image_bytes []byte, model_id string) *Company {
  //response structure
  type MicrosoftJson struct {
    Tags []struct {
      Label string  `json:"name"`
      Score float32 `json:"confidence"`
    } `json:"tags"`
    Adult struct {
      AdultScore float32 `json:"adultScore"`
      RacyScore  float32 `json:"racyScore"`
    } `json:"adult"`
    Categories []struct {
      Name   string `json:"name"`
      Detail struct {
        Celebrities []struct {
          Label string  `json:"name"`
          Score float32 `json:"confidence"`
        } `json:"celebrities"`
      } `json:"detail"`
    } `json:"categories"`
    Faces []struct {
      Bounding_Box struct {
        Left   int `json:"left"`
        Top    int `json:"top"`
        Width  int `json:"width"`
        Height int `json:"height"`
      } `json:"faceRectangle"`
      Age    int    `json:"age"`
      Gender string `json:"gender"`
    } `json:"faces"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10,
  }
  body := []byte(`{"url":"` + imgurl + `"}`)
  req, err := http.NewRequest("POST", "https://eastus2.api.cognitive.microsoft.com/vision/v1.0/analyze", bytes.NewBuffer(body))
  params := req.URL.Query()
  params.Add("language", "en")

  req.Header.Set("Ocp-Apim-Subscription-Key", secrets.Microsoft_Api_Key)
  req.Header.Set("Content-Type", "application/json")

  //vary based on type of request
  if model_id == "General" {
    params.Add("visualFeatures", "Tags")
  } else if model_id == "NSFW" {
    params.Add("visualFeatures", "Adult")
  } else if model_id == "Celebrity" {
    params.Add("visualFeatures", "Categories")
    params.Add("details", "Celebrities")
  } else if model_id == "Face" {
    params.Add("visualFeatures", "Faces")
  }
  req.URL.RawQuery = params.Encode()

  //make the request
  start := time.Now()
  resp, err := client.Do(req)
  elapsed := time.Since(start)
  if err != nil {
    panic(err)
  }

  //parse the response
  defer resp.Body.Close()
  data, _ := ioutil.ReadAll(resp.Body)
  fmt.Println(string(data))
  var dat MicrosoftJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }
  m := Company{Company: "Microsoft", Elapsed: elapsed.Seconds()}
  if model_id == "NSFW" {
    m.Tags = append(m.Tags, Tag{Label: "adultScore", Score: dat.Adult.AdultScore})
    m.Tags = append(m.Tags, Tag{Label: "racyScore", Score: dat.Adult.RacyScore})
  } else if model_id == "Celebrity" {
    for _, cat := range dat.Categories {
      if cat.Name == "people_" {
        for _, tag := range cat.Detail.Celebrities {
          m.Tags = append(m.Tags, Tag{Label: tag.Label, Score: tag.Score})
        }
      }
    }
  } else if model_id == "Face" {
    for _, tag := range dat.Faces {
      box := PixelBound{Top: tag.Bounding_Box.Top,
        Bottom: (tag.Bounding_Box.Top + tag.Bounding_Box.Height),
        Left:   tag.Bounding_Box.Left,
        Right:  tag.Bounding_Box.Left + tag.Bounding_Box.Width}
      m.Tags = append(m.Tags, Tag{Image: Pixel_Image_Crop(box, image_bytes), Description: tag.Gender})
    }
  } else {
    for _, tag := range dat.Tags {
      m.Tags = append(m.Tags, Tag{Label: tag.Label, Score: tag.Score})
    }
  }

  return &m
}

func client_amazon(image_bytes []byte, model_id string) *Company {
  //response structure
  //as long as I"m using the client to make the request, don"t strictly need this but
  //good to have it around as a template for how the response is structured
  type AmazonJson struct {
    Labels []struct {
      Label string  `json:"Label"`
      Score float32 `json:"Confidence"`
    } `json:"Labels"`
  }

  sess := session.Must(session.NewSession(&aws.Config{Region: aws.String("us-east-1")})) //&aws.Config{Region: aws.String("us-east-1"),}
  rek := rekognition.New(sess)
  var ml int64
  ml = 20
  a := Company{Company: "Amazon"}
  image := rekognition.Image{Bytes: image_bytes}
  if model_id == "General" {
    //set up input and make the timed request
    input := rekognition.DetectLabelsInput{Image: &image, MaxLabels: &ml}
    start := time.Now()
    resp, err := rek.DetectLabels(&input)
    elapsed := time.Since(start)
    if err != nil {
      panic(err)
    }
    //add the results to the company info
    a.Elapsed = elapsed.Seconds()
    for _, tag := range resp.Labels {
      a.Tags = append(a.Tags, Tag{Label: *tag.Name, Score: float32(*tag.Confidence)})
    }
  } else if model_id == "Celebrity" {
    input := rekognition.RecognizeCelebritiesInput{Image: &image}
    start := time.Now()
    resp, err := rek.RecognizeCelebrities(&input)
    elapsed := time.Since(start)
    if err != nil {
      panic(err)
    }
    a.Elapsed = elapsed.Seconds()
    for _, tag := range resp.CelebrityFaces {
      a.Tags = append(a.Tags, Tag{Label: *tag.Name, Score: float32(*tag.MatchConfidence)})
    }
  } else if model_id == "NSFW" {
    input := rekognition.DetectModerationLabelsInput{Image: &image}
    start := time.Now()
    resp, err := rek.DetectModerationLabels(&input)
    elapsed := time.Since(start)
    fmt.Println(resp)
    if err != nil {
      panic(err)
    }
    a.Elapsed = elapsed.Seconds()
    for _, tag := range resp.ModerationLabels {
      a.Tags = append(a.Tags, Tag{Label: *tag.Name, Score: float32(*tag.Confidence)})
    }
  } else if model_id == "Face" {
    input := rekognition.DetectFacesInput{Image: &image}
    start := time.Now()
    resp, err := rek.DetectFaces(&input)
    elapsed := time.Since(start)
    fmt.Println(resp)
    if err != nil {
      panic(err)
    }
    a.Elapsed = elapsed.Seconds()
    for _, tag := range resp.FaceDetails {
      box := RatioBound{Top: float32(*tag.BoundingBox.Top),
        Bottom: float32(*tag.BoundingBox.Top + *tag.BoundingBox.Height),
        Left:   float32(*tag.BoundingBox.Left),
        Right:  float32(*tag.BoundingBox.Left + *tag.BoundingBox.Width)}
      a.Tags = append(a.Tags, Tag{Image: Ratio_Image_Crop(box, image_bytes), Score: float32(*tag.Confidence)})
    }
  }

  return &a
}

func request_ibm(imgurl string, image_bytes []byte, model_id string) *Company {
  //response structure
  type IbmJson struct {
    Images []struct {
      Classifiers []struct {
        Tags []struct {
          Label string  `json:"class"`
          Score float32 `json:"score"`
        } `json:"classes"`
      } `json:"classifiers"`
    } `json:"images"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10,
  }
  body := []byte(`{"url":"` + imgurl + `"}`)
  req, err := http.NewRequest("POST", "https://gateway-a.watsonplatform.net/visual-recognition/api/v3/classify", bytes.NewBuffer(body))
  params := req.URL.Query()
  params.Add("api_key", secrets.Ibm_Api_Key)
  params.Add("version", "2016-05-20")
  req.URL.RawQuery = params.Encode()

  //make the request
  start := time.Now()
  resp, err := client.Do(req)
  elapsed := time.Since(start)
  if err != nil {
    panic(err)
  }

  //parse the response
  defer resp.Body.Close()
  data, _ := ioutil.ReadAll(resp.Body)
  var dat IbmJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }
  i := Company{Company: "IBM", Elapsed: elapsed.Seconds()}
  for _, tag := range dat.Images[0].Classifiers[0].Tags {
    i.Tags = append(i.Tags, Tag{Label: tag.Label, Score: tag.Score})
  }

  return &i
}
