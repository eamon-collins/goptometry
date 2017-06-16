/**
Eamon Collins
Attempt at porting the optometry tool into Golang
**/

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rekognition"
	"github.com/clarifai/clarifai-go"
	"github.com/eamon-collins/goptometry/secrets"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"
)

var port = ":8000"

type Company struct {
	Company string
	Elapsed float64
	Tags    []Tag
}

type Tag struct {
	Label string
	Score float32
}

type Results struct {
	Companies []Company
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", index)

	fmt.Println("Starting server at localhost", port)
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
		for _, comp := range r.Form["competitors"] {
			comp_map[comp] = true
	  }
		//make a base64 encoding of the image at the imgurl
		//amazon just wants normal bytes, but someone else might want base64
		image_res, err := http.Get(imgurl)
		if err != nil {
			panic(err)
		}
		defer image_res.Body.Close()
		image_bytes, _ := ioutil.ReadAll(image_res.Body)
		image_buf := new(bytes.Buffer)
		enc := base64.NewEncoder(base64.StdEncoding, image_buf)
		defer enc.Close()
		enc.Write(image_bytes)
		//base64_bytes := image_buf.Bytes()

		//the results array to write all applicable returned tags into
		//will be passed to the html once filled
		var results Results

		//CLARIFAI CLIENT PREDICT
		if comp_map["Clarifai"] {
			clarifai_predict := clarifai.NewClient(secrets.Clarifai_Client_ID, secrets.Clarifai_Client_Secret)
			start := time.Now()
			clarifai_resp, err := clarifai_predict.Tag(clarifai.TagRequest{URLs: []string{imgurl}})
			elapsed := time.Since(start)
			if err != nil {
				fmt.Println(err)
			}
			c := Company{Company: "Clarifai", Elapsed: elapsed.Seconds()}
			for i, label := range clarifai_resp.Results[0].Result.Tag.Classes {
				c.Tags = append(c.Tags, Tag{label, clarifai_resp.Results[0].Result.Tag.Probs[i]})
			}
			results.Companies = append(results.Companies, c)
		}

		//GOOGLE CLOUD VISION
		if comp_map["Google"] {
			results.Companies = append(results.Companies, request_google(imgurl))
		}
		//MICROSOFT AZURE VISUAL RECOGNITION
		if comp_map["Microsoft"] {
			results.Companies = append(results.Companies, request_microsoft(imgurl))
		}
		//AMAZON REKOGNITION
		if comp_map["Amazon"] {
			results.Companies = append(results.Companies, client_amazon(image_bytes))
		}

		fmt.Println(results)
		tmpl.ExecuteTemplate(w, "index", &results)
	}

}

func request_google(imgurl string) Company {
	//response structure
	type GoogleJson struct {
		Responses []struct {
			LabelAnnotations []struct {
				Label string  `json:"description"`
				Score float32 `json:"score"`
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
        "imageUri":"` + imgurl + `"}},
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

func request_microsoft(imgurl string) Company {
	//response structure
	type MicrosoftJson struct {
		Tags []struct {
			Label string  `json:"name"`
			Score float32 `json:"confidence"`
		} `json:"tags"`
	}

	//build the request
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	body := []byte(`{"url":"` + imgurl + `"}`)
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

func client_amazon(b64image []byte) Company {
	//response structure
	//as long as I'm using the client to make the request, don't strictly need this but
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
	image := rekognition.Image{Bytes: b64image}
	input := rekognition.DetectLabelsInput{Image: &image, MaxLabels: &ml}
	start := time.Now()
	resp, err := rek.DetectLabels(&input)
	elapsed := time.Since(start)
	if err != nil {
		panic(err)
	}
	a := Company{Company: "Amazon", Elapsed: elapsed.Seconds()}
	for _, tag := range resp.Labels {
		a.Tags = append(a.Tags, Tag{*tag.Name, float32(*tag.Confidence)})
	}
	fmt.Println(a)
	return a
}
