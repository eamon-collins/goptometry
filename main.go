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
	Clarifai  Company
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
	funcMap := template.FuncMap{
		"url": func(s string) template.URL {
			return template.URL(s)
		},
	}
	ip := filepath.Join("templates", "index.html")
	fp := filepath.Join("templates", "form.html")
  rp := filepath.Join("templates", "response.html")
	tmpl, err := template.New("template").Funcs(funcMap).ParseFiles(ip, fp, rp)
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	if r.Method == "GET" {
		tmpl.ExecuteTemplate(w, "index", nil)
	} else {
		r.ParseForm()

		//don"t make requests to any apis, just give prefetched data in format to test out the layout
		if r.FormValue("layouttest") == "layouttest" {
			var results Results
			results.Clarifai = Company{Company: "Clarifai", Elapsed: 1.322, Tags: []Tag{Tag{Label: "summer", Score: 0.98168695}, Tag{Label: "nature", Score: 0.97212166}, Tag{Label: "farm", Score: 0.9689047}, Tag{Label: "grass", Score: 0.95977676}, Tag{Label: "outdoors", Score: 0.9521229}, Tag{Label: "field", Score: 0.9091958}}}
			results.Companies = append(results.Companies, Company{Company: "Google", Elapsed: 2.332, Tags: []Tag{Tag{Label: "rural area", Score: 0.6766008}, Tag{Label: "farm", Score: 0.6482424}, Tag{Label: "meadow", Score: 0.59396565}, Tag{Label: "horse like mammal", Score: 0.51159537}}})
			results.Companies = append(results.Companies, Company{Company: "IBM", Elapsed: 2.109, Tags: []Tag{Tag{Label: "green color", Score: 0.963}, Tag{Label: "animal", Score: 0.765}}}) /** {"tag": u"mammal", "score": 0.653}, {"tag": u"domestic animal", "score": 0.653}, {"tag": u"dog", "score": 0.652}, {"tag": u"ruminant", "score": 0.603}, {"tag": u"deer", "score": 0.602}, {"tag": u"vizsla dog", "score": 0.569}, {"tag": u"Great Dane dog", "score": 0.558}, {"tag": u"person", "score": 0.55}, {"tag": u"boy at farm", "score": 0.549}, {"tag": u"palomino horse", "score": 0.53}]})
			  if amazon:
			    results.append({"company": "Amazon", "elapsed": 3.217, "tags": [{"tag": u"Human", "score": 99.30406951904297}, {"tag": u"People", "score": 99.30635070800781}, {"tag": u"Person", "score": 99.30635070800781}, {"tag": u"Backyard", "score": 77.3866958618164}, {"tag": u"Yard", "score": 77.3866958618164}, {"tag": u"Ivy", "score": 76.58114624023438}, {"tag": u"Plant", "score": 76.58114624023438}, {"tag": u"Vine", "score": 76.58114624023438}, {"tag": u"Shorts", "score": 75.4872055053711}, {"tag": u"Blossom", "score": 67.45735168457031}, {"tag": u"Flora", "score": 67.45735168457031}, {"tag": u"Flower", "score": 67.45735168457031}, {"tag": u"Herbal", "score": 63.904659271240234}, {"tag": u"Herbs", "score": 63.904659271240234}, {"tag": u"Planter", "score": 63.904659271240234}]})
			  if microsoft:
			    results.append({"company": "Microsoft", "elapsed": 2.698, "tags": [{"tag": u"tree", "score": 0.9998617172241211}, {"tag": u"outdoor", "score": 0.999527096748352}, {"tag": u"grass", "score": 0.9965176582336426}, {"tag": u"standing", "score": 0.8547968864440918}, {"tag": u"house", "score": 0.4055963158607483}]})
			  **/
			tmpl.ExecuteTemplate(w, "index", &results)
			return
		}

		imgurl := template.HTMLEscapeString(r.Form.Get("imgurl"))
		comp_map := make(map[string]bool)
		for _, comp := range r.Form["competitors"] {
			comp_map[comp] = true
		}
		model_id := r.Form.Get("model_id")
    fmt.Println(model_id)

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

		//the results object to write the responses to as they are passed to the channel
		//will be passed to the html once filled
		var results Results

		res_channel := make(chan *Company)

    //For each company result requested, start a function in a separate goroutine to retrieve that data
    //This allows all requests to be executed concurently rather than sequentially
    go func(imgurl string, model_id string) {
      res_channel <- request_clarifai(imgurl, model_id)
    }(imgurl, model_id)

		for key, _ := range comp_map {
			go func(imgurl string, image_bytes []byte, key string) {
				//GOOGLE CLOUD VISION
				if key == "Google" {
					res_channel <- request_google(imgurl)
				}
				//MICROSOFT AZURE VISUAL RECOGNITION
				if key == "Microsoft" {
					res_channel <- request_microsoft(imgurl)
				}
				//AMAZON REKOGNITION
				if key == "Amazon" {
					res_channel <- client_amazon(image_bytes)
				}
				//IBM VISUAL RECOGNITION
				if key == "IBM" {
					res_channel <- request_ibm(imgurl)
				}
			}(imgurl, image_bytes, key)
		}

		//waits for responses from the goroutines fetching the requested companies,
		//adds them to results.Companies as they come in.
		//tmpl.ExecuteTemplate(w, "index", nil)
		for {
			select {
			case r := <-res_channel:
				res := *r
				if res.Company == "Clarifai" {
					results.Clarifai = res
				} else {
					results.Companies = append(results.Companies, res)
				}
				if (len(results.Companies) == len(comp_map)) && (results.Clarifai.Company == "Clarifai") {
					//return html, later may do this each time a company is received to simulate
					//ajax calls, except it would reload whole result every time.
					tmpl.ExecuteTemplate(w, "index", &results)
					return
			  }
      case <-time.After(time.Second * 5):
        fmt.Println("Timeout")
        tmpl.ExecuteTemplate(w, "index", &results)
        return
			}
		}
	}

}

func client_clarifai(imgurl string) *Company {
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
	return &c
}

func request_clarifai(imgurl string, model_id string) *Company {
	//response structure
	type ClarifaiJson struct {
		Outputs []struct {
			Data struct {
				Concepts []struct {
					Label string  `json:"name"`
					Score float32 `json:"value"`
				} `json:"concepts"`
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
	var dat ClarifaiJson
	if err := json.Unmarshal(data, &dat); err != nil {
		panic(err)
	}
	c := Company{Company: "Clarifai", Elapsed: elapsed.Seconds()}
	for _, tag := range dat.Outputs[0].Data.Concepts {
		c.Tags = append(c.Tags, Tag{tag.Label, tag.Score})
	}

	return &c

}

func request_google(imgurl string) *Company {
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

	return &g
}

func request_microsoft(imgurl string) *Company {
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

	return &m
}

func client_amazon(b64image []byte) *Company {
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

	return &a
}

func request_ibm(imgurl string) *Company {
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
		i.Tags = append(i.Tags, Tag{tag.Label, tag.Score})
	}

	return &i
}
