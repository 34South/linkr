package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2/bson"
	"log"
	"net/http"
	"time"
)

type Link struct {
	ShortURL string `json:shortUrl`
	LongURL  string `json:longUrl`
}

type APIResponse struct {
	StatusMessage string `json:statusmessage`
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "GET /{shortUrl} to redirect, POST to create")
}

func AddHandler(w http.ResponseWriter, r *http.Request) {

	// Start link doc with the bits we don't get in request body
	ld := LinkDoc{
		ID: bson.NewObjectId(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Get the rest from req body...
	responseEncoder := json.NewEncoder(w)
	if err := json.NewDecoder(r.Body).Decode(&ld); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := responseEncoder.Encode(&APIResponse{StatusMessage: err.Error()}); err != nil {
			fmt.Fprintf(w, "Error occured while processing post request %v \n", err.Error())
		}
		return
	}

	err := MongoDB.AddLink(ld)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		if err := responseEncoder.Encode(&APIResponse{StatusMessage: err.Error()}); err != nil {
			fmt.Fprintf(w, "Error %s occured while trying to add the url \n", err.Error())
		}
		return
	}
	responseEncoder.Encode(&APIResponse{StatusMessage: "Ok"})
}

func RedirectHandler(w http.ResponseWriter, r *http.Request) {

	// Get short url from path
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]

	if len(sUrl) > 0 {

		// Get link doc from db
		ld, err := MongoDB.FindLink(sUrl)
		if err != nil {
			fmt.Fprintf(w, "Could not find a long url that corresponds to the short url %s \n", sUrl)
			return
		}

		// Increment Clicks
		go MongoDB.IncrementClicks(ld.ShortUrl)

		stats := LinkStatsDoc{
			ID:        bson.NewObjectId(),
			LinkID:    ld.ID,
			CreatedAt: time.Now(),
			Referrer:  r.Referer(),
			Agent:     r.UserAgent(),
		}

		// Increment Clicks
		err = MongoDB.RecordStats(stats)
		if err != nil {
			log.Println("Error recording stats:", err)
		}


		//Ensure we are dealing with an absolute path
		http.Redirect(w, r, ld.LongUrl, http.StatusFound)
	}
}
