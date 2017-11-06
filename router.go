package main

import (
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"log"
	"net/http"
	"os"
)

func Start() {

	r := mux.NewRouter()
	r.Methods("GET").Path("/").HandlerFunc(IndexHandler)
	r.Methods("GET").Path("/popular.html").HandlerFunc(PopularHTMLHandler)
	r.Methods("GET").Path("/popular.json").HandlerFunc(PopularJSONHandler)
	r.Methods("GET").Path("/latest.html").HandlerFunc(LatestHTMLHandler)
	r.Methods("GET").Path("/broken.json").HandlerFunc(BrokenJSONHandler)
	r.Methods("GET").Path("/{shortUrl}.json").HandlerFunc(JSONHandler)
	r.Methods("GET").Path("/{shortUrl}").HandlerFunc(RedirectHandler)

	// Heroku dyanmically assigns port so..
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	//... wrap r with simple CORS handler?
	h := cors.Default().Handler(r)

	log.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, h))
}
