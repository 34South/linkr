package main

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

func Start() {

	r := mux.NewRouter()
	r.Methods("GET").Path("/").HandlerFunc(IndexHandler)
	r.Methods("GET").Path("/{shortUrl}").HandlerFunc(RedirectHandler)
	// TODO: secure this !!
	r.Methods("POST").Path("/").HandlerFunc(AddHandler)

	// Heroku dyanmically assigns port so..
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Listening on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
