package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gopkg.in/mgo.v2/bson"
	"github.com/gorilla/mux"
)

type Link struct {
	ShortURL string `json:shortUrl`
	LongURL  string `json:longUrl`
}

type APIResponse struct {
	StatusMessage string `json:statusmessage`
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "404-s.html")
	//fmt.Fprint(w, "GET /{shortUrl} to redirect, POST to create")
}

func AddHandler(w http.ResponseWriter, r *http.Request) {

	// Start link doc with the bits we don't get in request body
	ld := LinkDoc{
		ID:        bson.NewObjectId(),
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
			http.ServeFile(w, r, "404-s.html")
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

		// Check link is UP, if it isn't we will still create the record but will set the status
		// This way we have a nice way to record broken links
		res, err := http.Get(ld.LongUrl)
		if err != nil {
			// 'res' is nil so set status here..
			log.Println("Error checking long url:", err)
			// Set status code - no server response at all / timeout
			stats.StatusCode = http.StatusServiceUnavailable
		} else {
			defer res.Body.Close()
			stats.StatusCode = res.StatusCode
		}
		// Don't refer to 'res' past here in case it is nil

		// Update LinkDoc if http status changes
		if stats.StatusCode != ld.LastStatusCode {
			// TODO - actually update the doc
			msg := fmt.Sprintf("Changing http status of %s from %v to %v", ld.ShortUrl, ld.LastStatusCode, stats.StatusCode)
			log.Println(msg)
			err := MongoDB.UpdateStatusCode(ld.ShortUrl, stats.StatusCode)
			if err != nil {
				log.Println("Error updating status code:", err)
			}
		}

		// Record LinkStats doc
		err = MongoDB.RecordStats(stats)
		if err != nil {
			log.Println("Error recording stats:", err)
		}

		if stats.StatusCode == 200 {
			http.Redirect(w, r, ld.LongUrl, http.StatusFound)
			return
		}

		http.ServeFile(w, r, "404-l.html")
	}
}
