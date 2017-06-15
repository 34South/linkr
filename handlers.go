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
	"gopkg.in/mgo.v2"
)

type Link struct {
	ShortURL string `json:shortUrl`
	LongURL  string `json:longUrl`
}

type APIResponse struct {
	StatusMessage string `json:statusmessage`
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/popular.html", http.StatusSeeOther)
}

// RedirectHandler redirects the request to the target long url
func RedirectHandler(w http.ResponseWriter, r *http.Request) {

	// Get short url from path
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]

	if len(sUrl) > 0 {

		fmt.Printf("Looking for %s... ", sUrl)

		// Get link doc from db
		ld, err := MongoDB.FindLink(sUrl)
		if err == mgo.ErrNotFound {
			fmt.Println("not found")
			msg := fmt.Sprintf("The link /%s could not be found in the database.", sUrl)
			tpl.ExecuteTemplate(w, "error", msg)
			return
		}
		// Some other db error...
		if err != nil {
			fmt.Println("error")
			msg := fmt.Sprintf("The server has encountered an error whilst trying to look up the link /%s", sUrl)
			tpl.ExecuteTemplate(w, "error", msg)
			return
		}
		// Found
		fmt.Println(" -> ", ld.LongUrl)

		// Increment clicks regardless... a click is a click
		go MongoDB.IncrementClicks(ld.ShortUrl)

		// If the last status was 200 - OK redirect immediately to save time. If the subsequent check finds the link is
		// broken then only the first user will see the "hang" or 404. Subsequent users will see the direct link page.
		// This is a much faster user experience as the url check can happen AFTER.
		if ld.LastStatusCode == 200 {
			http.Redirect(w, r, ld.LongUrl, http.StatusSeeOther)

		} else {
			tpl.ExecuteTemplate(w, "direct", ld.LongUrl)
		}

		// Either way the user gets a result quickly, and we can check the link AFTER that fact...
		// DO NOT RESPOND PAST HERE!!


		// LinkStats
		stats := LinkStatsDoc{
			ID:        bson.NewObjectId(),
			LinkID:    ld.ID,
			CreatedAt: time.Now(),
			Referrer:  r.Referer(),
			Agent:     r.UserAgent(),
		}

		// Check link is UP, if it isn't we can record the status. Note that this fancy client
		// function is here because one link had more than 10 redirects at the remote end.
		// So this allows us to up the limit (10 is Go default)... it came from here:
		// https://gist.github.com/VojtechVitek/eb0171fc65f945a8641e
		client := &http.Client{
			CheckRedirect: func() func(req *http.Request, via []*http.Request) error {
				redirects := 0
				return func(req *http.Request, via []*http.Request) error {
					if redirects > 15 {
						fmt.Println("Checking target url had %v redirects", redirects)
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
			fmt.Println("Error checking long url:", err)
			// No server response / timeout
			stats.StatusCode = http.StatusServiceUnavailable

		} else {
			defer res.Body.Close()
			stats.StatusCode = res.StatusCode
		}
		// Don't refer to 'res' past here in case it is nil

		// Update LinkDoc if http status changes
		if stats.StatusCode != ld.LastStatusCode {
			msg := fmt.Sprintf("Changing http status of %s from %v to %v\n", ld.ShortUrl, ld.LastStatusCode, stats.StatusCode)
			fmt.Println(msg)
			err := MongoDB.UpdateStatusCode(ld.ShortUrl, stats.StatusCode)
			if err != nil {
				fmt.Println("Error updating status code:", err)
			}
		}

		// Record LinkStats doc
		err = MongoDB.RecordStats(stats)
		if err != nil {
			fmt.Println("Error recording stats:", err)
		}
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
		w.Header().Set("Cache-Control", "no-cache, no-store, private, max-age=0")
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
	w.Header().Set("Cache-Control", "no-cache, no-store, private, max-age=0")
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
	w.Header().Set("Cache-Control", "no-cache, no-store, private, max-age=0")
	w.Write(js.([]byte))
}
