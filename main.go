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
	Label       string
	Score       float32
	Description string
	Image       string //to display cropped sections next to logo results and face results
}

type Results struct {
	Clarifai  Company
	Companies []Company
	Image     string //to display the original image at the top of the screen
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

		imgurl := template.HTMLEscapeString(r.Form.Get("imgurl"))
		comp_map := make(map[string]bool)
		for _, comp := range r.Form["competitors"] {
			comp_map[comp] = true
		}
		model_id := r.Form.Get("model_id")
		fmt.Println(model_id)

		//make a base64 encoding of the image at the imgurl
		//amazon just wants normal bytes, base64 is for displaying
		image_res, err := http.Get(imgurl)
		if err != nil {
			panic(err)
		}
		defer image_res.Body.Close()
		image_bytes, _ := ioutil.ReadAll(image_res.Body)
		base64_string := base64.StdEncoding.EncodeToString(image_bytes)

		//don"t make requests to any apis, just give prefetched data in format to test out the layout
		if r.FormValue("layouttest") == "layouttest" {
			var results Results
			results.Image = base64_string
			results.Clarifai = Company{Company: "Clarifai", Elapsed: 1.322, Tags: []Tag{Tag{Label: "summer", Score: 0.98168695}, Tag{Label: "nature", Score: 0.97212166}, Tag{Label: "farm", Score: 0.9689047}, Tag{Label: "grass", Score: 0.95977676}, Tag{Label: "outdoors", Score: 0.9521229}, Tag{Label: "field", Score: 0.9091958}}}
			results.Companies = append(results.Companies, Company{Company: "Google", Elapsed: 2.332, Tags: []Tag{Tag{Label: "rural area", Score: 0.6766008}, Tag{Label: "farm", Score: 0.6482424}, Tag{Label: "meadow", Score: 0.59396565}, Tag{Label: "horse like mammal", Score: 0.51159537}}})
			results.Companies = append(results.Companies, Company{Company: "IBM", Elapsed: 2.109, Tags: []Tag{Tag{Label: "green color", Score: 0.963}, Tag{Label: "animal", Score: 0.765}}})
			tmpl.ExecuteTemplate(w, "index", &results)
			return
		}

		//the results object to write the responses to as they are passed to the channel
		//will be passed to the html once filled
		var results Results
		//add the image being analyzed in base64 form
		results.Image = base64_string

		res_channel := make(chan *Company)

		//For each company result requested, start a function in a separate goroutine to retrieve that data
		//This allows all requests to be executed concurently rather than sequentially
		go func(imgurl string, image_bytes []byte, model_id string) {
			res_channel <- request_clarifai(imgurl, image_bytes, model_id)
		}(imgurl, image_bytes, model_id)

		for key, _ := range comp_map {
			go func(imgurl string, image_bytes []byte, key string) {
				//GOOGLE CLOUD VISION
				if key == "Google" {
					res_channel <- request_google(imgurl, image_bytes, r.Form.Get("google-model"))
				}
				//MICROSOFT AZURE VISUAL RECOGNITION
				if key == "Microsoft" {
					res_channel <- request_microsoft(imgurl, image_bytes, r.Form.Get("microsoft-model"))
				}
				//AMAZON REKOGNITION
				if key == "Amazon" {
					res_channel <- client_amazon(image_bytes, r.Form.Get("amazon-model"))
				}
				//IBM VISUAL RECOGNITION
				if key == "IBM" {
					res_channel <- request_ibm(imgurl, image_bytes, r.Form.Get("ibm-model"))
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
			case <-time.After(time.Second * 15):
				fmt.Println("Timeout")
				tmpl.ExecuteTemplate(w, "index", &results)
				return
			}
		}
	}

}

func client_clarifai(imgurl string, image_bytes []byte) *Company {
	clarifai_predict := clarifai.NewClient(secrets.Clarifai_Client_ID, secrets.Clarifai_Client_Secret)
	start := time.Now()
	clarifai_resp, err := clarifai_predict.Tag(clarifai.TagRequest{URLs: []string{imgurl}})
	elapsed := time.Since(start)
	if err != nil {
		fmt.Println(err)
	}
	c := Company{Company: "Clarifai", Elapsed: elapsed.Seconds()}
	for i, label := range clarifai_resp.Results[0].Result.Tag.Classes {
		c.Tags = append(c.Tags, Tag{Label: label, Score: clarifai_resp.Results[0].Result.Tag.Probs[i]})
	}
	return &c
}

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
			box := ClarifaiBound{Top: logo.Region_Info.Bounding_Box.Top,
				Bottom: logo.Region_Info.Bounding_Box.Bottom,
				Left:   logo.Region_Info.Bounding_Box.Left,
				Right:  logo.Region_Info.Bounding_Box.Right}
			c.Tags = append(c.Tags, Tag{Label: logo.Data.Concepts[0].Label, Image: Clarifai_Image_Crop(box, image_bytes), Score: logo.Data.Concepts[0].Score})
		}
	} else if model_id == "a403429f2ddf4b49b307e318f00e528b" { //Face detection
		for _, face := range dat.Outputs[0].Data.Regions {
			box := ClarifaiBound{Top: face.Region_Info.Bounding_Box.Top,
				Bottom: face.Region_Info.Bounding_Box.Bottom,
				Left:   face.Region_Info.Bounding_Box.Left,
				Right:  face.Region_Info.Bounding_Box.Right}
			fmt.Println(box)
			c.Tags = append(c.Tags, Tag{Image: Clarifai_Image_Crop(box, image_bytes)})
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
				Label string  `json:"description"`
				Score float32 `json:"score"`
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
      box := GoogleBound{Top: tag.BoundingPoly.Vertices[0].Y,
        Bottom: tag.BoundingPoly.Vertices[2].Y,
        Left:   tag.BoundingPoly.Vertices[0].X,
        Right:  tag.BoundingPoly.Vertices[2].X}
			g.Tags = append(g.Tags, Tag{Label: tag.Label, Score: tag.Score, Image: Google_Image_Crop(box, image_bytes)})
		}
	} else if model_id == "Face" {
		for _, tag := range dat.Responses[0].FaceAnnotations {
			box := GoogleBound{Top: tag.BoundingPoly.Vertices[0].Y,
				Bottom: tag.BoundingPoly.Vertices[2].Y,
				Left:   tag.BoundingPoly.Vertices[0].X,
				Right:  tag.BoundingPoly.Vertices[2].X}
			g.Tags = append(g.Tags, Tag{Image: Google_Image_Crop(box, image_bytes), Score: tag.Score})
		}
	}
  fmt.Println(g)
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
      Bounding_Box struct{
        Left int `json:"left"`
        Top int `json:"top"`
        Width int `json:"width"`
        Height int `json:"height"`
      } `json:"faceRectangle"`
      Age int `json:"age"`
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
	}else if model_id == "Face" {
    for _, tag := range dat.Faces {
      box := GoogleBound{Top: tag.Bounding_Box.Top,
        Bottom: (tag.Bounding_Box.Top + tag.Bounding_Box.Height),
        Left: tag.Bounding_Box.Left,
        Right: tag.Bounding_Box.Left + tag.Bounding_Box.Width}
      m.Tags = append(m.Tags, Tag{Image: Google_Image_Crop(box, image_bytes), Description: tag.Gender})
    }
  }else {
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
