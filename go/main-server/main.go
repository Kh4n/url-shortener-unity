package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path"
	"strconv"

	"github.com/Kh4n/url-shortener-unity/go/util"
)

type setOrShortenResponse struct {
	Succeeded bool   `json:"succeeded"`
	ErrorMsg  string `json:"errorMsg"`

	ShortenedURL string `json:"shortenedURL"`
	OriginalURL  string `json:"originalURL"`
}

type reserveResponse struct {
	Succeeded bool   `json:"succeeded"`
	ErrorMsg  string `json:"errorMsg"`

	Keys []string `json:"keys"`
}

func redirect(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	if key == "/" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}
	url, err := store.Query(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func shorten(w http.ResponseWriter, r *http.Request) {
	resp := setOrShortenResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	urlStr := r.Form.Get("url")
	key, err := store.Store(urlStr)
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
		resp.ShortenedURL = r.Host + "/" + key
		resp.OriginalURL = urlStr
	}
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func set(w http.ResponseWriter, r *http.Request) {
	resp := setOrShortenResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	key := r.Form.Get("key")
	urlStr := r.Form.Get("url")
	err = store.Set(key, urlStr)
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
		resp.ShortenedURL = r.Host + "/" + key
		resp.OriginalURL = urlStr
	}
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func reserve(w http.ResponseWriter, r *http.Request) {
	resp := reserveResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	num, err := strconv.ParseUint(r.Form.Get("num"), 10, 13)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}
	resp.Keys, err = store.Reserve(int(num))
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
	}
	js, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

var store *util.URLStore

func main() {
	var err error
	store, err = util.NewURLStore("./db")
	if err != nil {
		log.Fatalf("Unable to create database: %s\n", err.Error())
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", redirect)
	mux.HandleFunc("/shorten", shorten)
	mux.HandleFunc("/set", set)
	mux.HandleFunc("/reserve", reserve)
	log.Println("Starting server on :8080")
	err = http.ListenAndServe(":8080", mux)
	log.Fatal(err)
}
