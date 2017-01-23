package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"errors"
	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2/bson"
	"html/template"
	"os"
	"strconv"
)

type Link struct {
	ShortURL string `json:shortUrl`
	LongURL  string `json:longUrl`
}

type APIResponse struct {
	StatusMessage string `json:statusmessage`
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "error_s.html")
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

// RedirectHandler redirects the request to the target long url
func RedirectHandler(w http.ResponseWriter, r *http.Request) {

	// Get short url from path
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]

	if len(sUrl) > 0 {

		// Get link doc from db
		ld, err := MongoDB.FindLink(sUrl)
		if err != nil {
			http.ServeFile(w, r, "error_s.html")
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

		// Check link is UP, if it isn't we can record the status. Note that this fancy client
		// function is here because one link had more than 10 redirects at th remote end.
		// So this allows us to up the limit (10 is Go default)... it came from here:
		// https://gist.github.com/VojtechVitek/eb0171fc65f945a8641e
		client := &http.Client{
			CheckRedirect: func() func(req *http.Request, via []*http.Request) error {
				redirects := 0
				return func(req *http.Request, via []*http.Request) error {
					if redirects > 15 {
						log.Printf("Checking target url has had %v redirects", redirects)
						return errors.New("More than 15 redirects")
					}
					redirects++
					return nil
				}
			}(),
		}

		res, err := client.Get(ld.LongUrl)
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

		if stats.StatusCode == http.StatusOK {
			http.Redirect(w, r, ld.LongUrl, http.StatusFound)
			return
		}

		http.ServeFile(w, r, "error_l.html")
	}
}

// JSONHandler responds with the JSON info about the link
func JSONHandler(w http.ResponseWriter, r *http.Request) {

	// Get short url from path
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]

	if len(sUrl) > 0 {

		// Get link doc from db
		ld, err := MongoDB.FindLink(sUrl)
		if err != nil {
			http.ServeFile(w, r, "error_s.html")
			return
		}

		var js interface{}
		js, err = json.Marshal(ld)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Write(js.([]byte))

		return
	}
}

// Popular shows the most popular links
func PopularJSONHandler(w http.ResponseWriter, r *http.Request) {

	ld, err := MongoDB.Popular(10)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var js interface{}
	js, err = json.Marshal(ld)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(js.([]byte))
}

// PopularHTMLHandler shows the most popular links in an HTML template
func PopularHTMLHandler(w http.ResponseWriter, r *http.Request) {

	// Get n from the url if there, otherwise default to 10
	q := r.URL.Query()
	ns, ok := q["n"] /// n is a slice
	limit := 10      // default
	var err error
	if ok {
		limit, err = strconv.Atoi(ns[0])
		if err != nil {
			limit = 10 // if the number in query string is bung
		}
	}

	// Get the link docs
	ld, err := MongoDB.Popular(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the template
	t, err := template.ParseFiles("popular.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set up some page data
	pageData := make(map[string]interface{})
	pageData["Title"] = "Popular Links"
	pageData["Heading"] = fmt.Sprintf("%v Most Popular Links", limit)
	pageData["BaseUrl"] = os.Getenv("LINKR_BASE_URL")
	pageData["Links"] = ld

	// Serve it up
	err = t.Execute(w, pageData)
	if err != nil {
		log.Printf("template execution: %s", err)
	}
}

// BrokenJSONHandler shows links with LastStatsCode other than 200
func BrokenJSONHandler(w http.ResponseWriter, r *http.Request) {

	ld, err := MongoDB.Broken()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var js interface{}
	js, err = json.Marshal(ld)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Write(js.([]byte))
}
