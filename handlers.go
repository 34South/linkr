package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
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
	reqBodyStruct := new(Link)
	responseEncoder := json.NewEncoder(w)
	if err := json.NewDecoder(r.Body).Decode(&reqBodyStruct); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if err := responseEncoder.Encode(&APIResponse{StatusMessage: err.Error()}); err != nil {
			fmt.Fprintf(w, "Error occured while processing post request %v \n", err.Error())
		}
		return
	}
	err := MongoDB.AddUrl(reqBodyStruct.LongURL, reqBodyStruct.ShortURL)
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
	//retrieve the variable from the request
	vars := mux.Vars(r)
	sUrl := vars["shortUrl"]
	if len(sUrl) > 0 {
		//find long url that corresponds to the short url
		lUrl, err := MongoDB.FindLongUrl(sUrl)
		if err != nil {
			fmt.Fprintf(w, "Could not find a long url that corresponds to the short url %s \n", sUrl)
			return
		}

		// Increment Clicks
		go MongoDB.IncrementClicks(sUrl)

		// TODO - record stats in a separate collection

		// Other stats
		fmt.Println("Date time:", time.Now())
		fmt.Println("Agent:", r.UserAgent())
		fmt.Println("Referrer:", r.Referer())

		//Ensure we are dealing with an absolute path
		http.Redirect(w, r, lUrl, http.StatusFound)
	}
}
