/**
Eamon Collins
Attempt at porting the optometry tool into Golang
**/

package main

import (
  "net/http"
  "fmt"
  "reflect"
  "time"
  "bytes"
  "encoding/json"
  "encoding/base64"
  "io/ioutil"
  "html/template"
  "path/filepath"
  "github.com/clarifai/clarifai-go"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/rekognition"
  "github.com/eamon-collins/goptometry/secrets"
)

var port = ":8000"


type Company struct{
  Company string
  Elapsed float64
  Tags []Tag
}

type Tag struct{
  Label string
  Score float32
}

func main() {
  fs := http.FileServer(http.Dir("static"))
  http.Handle("/static/", http.StripPrefix("/static/", fs))
  
  http.HandleFunc("/", index)

  fmt.Println("Starting server at localhost",port)
  http.ListenAndServe(port, nil)
}

func index(w http.ResponseWriter, r *http.Request) {
  ip := filepath.Join("templates", "index.html")
  tp := filepath.Join("templates", "template.html")
  tmpl, err := template.ParseFiles(ip, tp)
  if err != nil {
    fmt.Println(err)
    panic(err)
  }

  if r.Method == "GET" {
    tmpl.ExecuteTemplate(w, "index", nil)
  } else {
    r.ParseForm()
    imgurl := template.HTMLEscapeString(r.Form.Get("imgurl"))
    comp_map := make(map[string]bool)
    for _, comp := range r.Form["competitors"]{
      comp_map[comp] = true
    }
    //make a base64 encoding of the image at the imgurl
    image_res, err := http.Get(imgurl)
    if err != nil{
      panic(err)
    }
    image_bytes, _ := ioutil.ReadAll(image_res.Body)
    fmt.Println(reflect.TypeOf(image_bytes))
    var base64_bytes []byte
    base64.StdEncoding.Encode(base64_bytes, image_bytes)

    var results []Company

    //CLARIFAI CLIENT PREDICT
    if comp_map["Clarifai"]{
      clarifai_predict := clarifai.NewClient(secrets.Clarifai_Client_ID, secrets.Clarifai_Client_Secret)
      start := time.Now()
      clarifai_resp, err := clarifai_predict.Tag(clarifai.TagRequest{URLs:[]string{imgurl}})
      elapsed := time.Since(start)
      if err != nil {
        fmt.Println(err)
      } else {
        fmt.Printf("%+v\n", clarifai_resp.Results[0].Result)
      }
      c := Company{Company: "Clarifai", Elapsed: elapsed.Seconds()}
      for i, label := range clarifai_resp.Results[0].Result.Tag.Classes {
        c.Tags = append(c.Tags, Tag{label, clarifai_resp.Results[0].Result.Tag.Probs[i]})
      }
      results = append(results, c)
    }

    //GOOGLE CLOUD VISION 
    if comp_map["Google"]{
      results = append(results, request_google(imgurl))
    }
    //MICROSOFT AZURE VISUAL RECOGNITION
    if comp_map["Microsoft"]{
      results = append(results, request_microsoft(imgurl))
    }
    //AMAZON REKOGNITION
    if comp_map["Amazon"]{
      results = append(results, client_amazon(base64_bytes))
    }
  }
  
}

func request_google(imgurl string) Company{
  //response structure
  type GoogleJson struct{
    Responses []struct{
      LabelAnnotations []struct{
        Label string `json:"description"`
        Score float32  `json:"score"`
      } `json:"labelAnnotations"`
    } `json:"responses"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10, 
  }
  body := []byte(`{requests:[{
    "image":{
      "source":{
        "imageUri":"`+imgurl+`"}},
        "features":[{
          "type":"LABEL_DETECTION",
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
  var dat GoogleJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }

  g := Company{Company: "Google", Elapsed: elapsed.Seconds()}
  for _, tag := range dat.Responses[0].LabelAnnotations {
    g.Tags = append(g.Tags, Tag{tag.Label, tag.Score})
  }

  return g
}

func request_microsoft(imgurl string) Company{
  //response structure
  type MicrosoftJson struct{
    Tags []struct{
        Label string `json:"name"`
        Score float32 `json:"confidence"`
      } `json:"tags"`
  }

  //build the request
  client := &http.Client{
    Timeout: time.Second * 10, 
  }
  body := []byte(`{"url":"`+imgurl+`"}`)
  req, err := http.NewRequest("POST", "https://eastus2.api.cognitive.microsoft.com/vision/v1.0/analyze", bytes.NewBuffer(body))
  params := req.URL.Query()
  params.Add("language", "en")
  params.Add("visualFeatures", "Tags")
  req.URL.RawQuery = params.Encode()
  req.Header.Set("Ocp-Apim-Subscription-Key", secrets.Microsoft_Api_Key)
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
  var dat MicrosoftJson
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }
  m := Company{Company: "Microsoft", Elapsed: elapsed.Seconds()}
  for _, tag := range dat.Tags {
    m.Tags = append(m.Tags, Tag{tag.Label, tag.Score})
  }

  return m
}

func client_amazon(b64image []byte) Company{
  sess := session.Must(session.NewSession())//&aws.Config{Region: aws.String("us-east-1"),}
  rek :=rekognition.New(sess)
  var ml int64
  ml = 20
  image := rekognition.Image{Bytes: b64image}
  input := rekognition.DetectLabelsInput{Image:&image, MaxLabels:&ml}
  resp, _ := rek.DetectLabels(&input)
  fmt.Println(resp)
  return *new(Company)
}