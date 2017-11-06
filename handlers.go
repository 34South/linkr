package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const maxRedirects = 30

const defaultResultCount = 20

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

		// The link has an active field that is set to 'false'
		if ld.Active == false {
			fmt.Println("inactive")
			msg := fmt.Sprintf("The link /%s is not currently active", sUrl)
			tpl.ExecuteTemplate(w, "error", msg)
			return
		}

		// Found
		fmt.Println(" -> ", ld.LongUrl)

		// Increment clicks regardless... a click is a click
		go MongoDB.IncrementClicks(ld.ShortUrl)

		// Check URL in a Go routine so no waiting... previously we waited and if the site was good the lastStatusCode
		// was changed before redirecting the user. The issue was that some sites had many redirects so the check took
		// a long time, then the actual redirect took a long time - painful. So now the check is done independently
		// and if there was an issue previously the user is shown a direct link straight away.
		go checkURL(r, &ld)

		// If the last status was 200 - OK, or 0 for first access, redirect immediately to save time.
		// If the subsequent check finds the link is broken then only the first user will see the "hang" or 404.
		// Subsequent users will see the direct link page. This is a faster user experience as the url check happens
		// is independent (see above).
		if ld.LastStatusCode == 200 || ld.LastStatusCode == 0 {
			http.Redirect(w, r, ld.LongUrl, http.StatusSeeOther)
			return
		}

		tpl.ExecuteTemplate(w, "direct", ld.LongUrl)
	}
}

func checkURL(r *http.Request, ld *LinkDoc) {

	fmt.Println("Go routine checking URL ", ld.LongUrl)

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

	// This stuff was an attempt to work around some remote servers resetting the connection from the
	// Go http client, however the urls were fine from a browser.
	//cfg := &tls.Config{
	//	MinVersion: tls.VersionTLS12,
	//	//MinVersion:         0,
	//	InsecureSkipVerify: true,
	//}
	//var netTransport = &http.Transport{
	//	//Dial: (&net.Dialer{
	//	//	Timeout: 10 * time.Second,
	//	//}).Dial,
	//	//TLSHandshakeTimeout: 10 * time.Second,
	//	TLSClientConfig: cfg,
	//}

	client := &http.Client{
		Timeout: time.Second * 30,
		//Transport: netTransport,
		CheckRedirect: func() func(req *http.Request, via []*http.Request) error {
			redirects := 0
			return func(req *http.Request, via []*http.Request) error {
				if redirects > maxRedirects {
					fmt.Printf("Checking target url had %v redirects\n", redirects)
					msg := fmt.Sprintf("More than %v redirects", maxRedirects)
					return errors.New(msg)
				}
				redirects++
				return nil
			}
		}(),
	}

	//res, err := client.Head(ld.LongUrl)
	res, err := client.Get(ld.LongUrl)
	if err != nil {

		// 'res' is nil so set status here..
		fmt.Println("Error checking long url:", err)

		// No server response / timeout
		stats.StatusCode = http.StatusGatewayTimeout

		err = MongoDB.UpdateStatusCode(ld.ShortUrl, stats.StatusCode)
		if err != nil {
			fmt.Println("Error updating status code:", err)
		}

		// Record LinkStats doc
		err = MongoDB.RecordStats(stats)
		if err != nil {
			fmt.Println("Error recording stats:", err)
		}

		return
	}

	// Got a response
	defer res.Body.Close()
	fmt.Println("HTTP Response: ", res.Status)
	stats.StatusCode = res.StatusCode

	// Update lastStatusCode in Link if it is unset (0) or changed
	if ld.LastStatusCode == 0 || stats.StatusCode != ld.LastStatusCode {

		msg := fmt.Sprintf("Updating last status code for %s from %v to %v\n", ld.ShortUrl, ld.LastStatusCode, stats.StatusCode)
		fmt.Println(msg)

		err = MongoDB.UpdateStatusCode(ld.ShortUrl, stats.StatusCode)
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

// JSONHandler responds with the JSON info about the link
func JSONHandler(w http.ResponseWriter, r *http.Request) {

	// Get short url from path
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]

	if len(sUrl) > 0 {

		// Get link doc from db
		ld, err := MongoDB.FindLink(sUrl)
		if err != nil {
			fmt.Println("not found")
			msg := fmt.Sprintf("The link /%s could not be found in the database.", sUrl)
			tpl.ExecuteTemplate(w, "error", msg)
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

	ld, err := MongoDB.Popular(defaultResultCount)
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

	// Get n from the url if there, otherwise default to defaultResultCount
	q := r.URL.Query()
	ns, ok := q["n"] /// n is a slice
	limit := defaultResultCount      // default
	var err error
	if ok {
		limit, err = strconv.Atoi(ns[0])
		if err != nil {
			limit = defaultResultCount // if the number in query string is bung
		}
	}

	// Get the link docs
	ld, err := MongoDB.Popular(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Parse the template
	//t, err := template.ParseFiles("popular.html")
	//if err != nil {
	//	http.Error(w, err.Error(), http.StatusInternalServerError)
	//	return
	//}

	// Set up some page data
	pageData := make(map[string]interface{})
	pageData["Title"] = "Popular Links"
	pageData["Heading"] = fmt.Sprintf("%v Most Popular Links", limit)
	pageData["BaseUrl"] = os.Getenv("LINKR_BASE_URL")
	pageData["Links"] = ld

	// Serve it up
	err = tpl.ExecuteTemplate(w, "popular", pageData)
	if err != nil {
		log.Printf("template execution: %s", err)
	}
}

// LatestHTMLHandler shows recently added links in an HTML template
func LatestHTMLHandler(w http.ResponseWriter, r *http.Request) {

	// Get n from the url if there, otherwise default to defaultResultCount
	q := r.URL.Query()
	ns, ok := q["n"] /// n is a slice
	limit := defaultResultCount      // default
	var err error
	if ok {
		limit, err = strconv.Atoi(ns[0])
		if err != nil {
			limit = defaultResultCount // if the number in query string is bung
		}
	}

	// Get the latest Resources
	rd, err := MongoDB.Latest(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set up some page data
	pageData := make(map[string]interface{})
	pageData["Title"] = "Latest Resources"
	pageData["Heading"] = fmt.Sprintf("%v Latest Resources", limit)
	pageData["Resources"] = rd

	// Serve it up
	err = tpl.ExecuteTemplate(w, "latest", pageData)
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
