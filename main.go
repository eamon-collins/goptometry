/**
Eamon Collins
Attempt at porting the optometry tool into Golang
**/

package main

import (
  "net/http"
  "fmt"
  "time"
  "bytes"
  "encoding/json"
  "io/ioutil"
  "html/template"
  "path/filepath"
  "github.com/clarifai/clarifai-go"
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
    fmt.Println(build_google(imgurl))
  }

  
}

func build_google(imgurl string) Company{
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
  var dat map[string]interface{}
  if err := json.Unmarshal(data, &dat); err != nil {
    panic(err)
  }
  fmt.Println(dat)
  g := Company{Company: "Google", Elapsed: elapsed.Seconds()}

  return g

}