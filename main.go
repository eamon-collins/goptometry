/**
Eamon Collins
Attempt at porting the optometry tool into Golang
**/

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var port = ":8000"

type Company struct {
	Company string
	Elapsed float64
	Tags    []Tag
  Model string
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
	Archive   bool   //tells javascript whether or not to preserve the html of the response for the archive
}

func main() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	http.HandleFunc("/", index)
	http.HandleFunc("/writearchive/", write_to_file)
	http.HandleFunc("/resetarchive/", reset_archive)

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
		//keep original image_bytes so cropping/boundingBoxes returned by
		//apis will remain consistent, but resize the picture to display
		//mostly for the purposes of the archive.
		image_bytes, _ := ioutil.ReadAll(image_res.Body)
		base64_string := Resize_Initial_Image(image_bytes)

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
		//set whether or not the screen should be snapshot for the archive
		results.Archive = (r.FormValue("archive") == "archive")
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

func write_to_file(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		f, err := os.OpenFile("static/archive.html", os.O_APPEND|os.O_WRONLY, 0666)
		if err != nil {
			panic(err)
		}
		_, err = f.Write(body)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}

//reset the archive to empty. Will not clear aggregate results.
func reset_archive(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		body := `<head><link href="/static/css/bootstrap-theme.min.css" type="text/css" rel="stylesheet"/>
    <link href="/static/css/bootstrap.min.css" type="text/css" rel="stylesheet"/>
    <link href="/static/css/extra.css" type="text/css" rel="stylesheet"/>
    <link href="https://fonts.googleapis.com/css?family=Roboto+Mono:500" rel="stylesheet" type="text/css">
    <script type="text/javascript" src="/static/js/jquery-3.2.1.min.js"></script>
    <script type="text/javascript" src="/static/js/bootstrap.min.js"></script>
    <script type="text/javascript" src="/static/js/tweaks.js"></script>
  </head><div id="currently-archive" style="display: none;"></div><button type="button" id="reset-archive" class="btn btn-danger">Reset Archive</button>
  <br/>`
		f, err := os.OpenFile("static/archive.html", os.O_TRUNC|os.O_WRONLY, 0666)
		if err != nil {
			panic(err)
		}
		_, err = f.WriteString(body)
		if err != nil {
			panic(err)
		}
		f.Close()
	}
}

//upon each request, update the log with the new averages
func update_aggregate_time() {

}
