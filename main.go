/**
Eamon Collins
Attempt at porting the optometry tool into Golang
**/

package main

import (
  "net/http"
  "fmt"
  "html/template"
  "path/filepath"
)

const port = ":8000"


type company struct{
  company string
  elapsed float
  tags []tag
}

type tag struct{
  label string
  score float
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
  //fp := filepath.Join("templates", filepath.Clean(r.URL.Path))

  results := []company
  

  tmpl, err := template.ParseFiles(ip)
  if err != nil {
    fmt.Println(err)
    panic(err)
  }
  tmpl.ExecuteTemplate(w, "index", )
}