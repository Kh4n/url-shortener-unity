package main

import (
	"fmt"
	"log"
	"net/http"
	"path"
	"strconv"

	"github.com/Kh4n/url-shortener-unity/go/util"
)

type MainServer struct {
	store *util.URLStore
	mux   *http.ServeMux
	port  uint32
}

func NewMainServer(dbLocation string, port uint32) (*MainServer, error) {
	ret := &MainServer{
		mux:  http.NewServeMux(),
		port: port,
	}
	var err error
	ret.store, err = util.NewURLStore(dbLocation)
	if err != nil {
		return nil, err
	}

	ret.mux.HandleFunc("/", ret.redirect)
	ret.mux.HandleFunc("/shorten", ret.shorten)
	ret.mux.HandleFunc("/query", ret.query)

	ret.mux.HandleFunc("/reserve", ret.reserve)
	ret.mux.HandleFunc("/setReserve", ret.setReserve)

	return ret, nil
}

func (ms *MainServer) Start() error {
	log.Println("Starting server on :8080")
	return http.ListenAndServe(fmt.Sprintf(":%d", ms.port), ms.mux)
}

func (ms *MainServer) redirect(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	if key == "/" {
		w.Write([]byte("create a post request to /shorten to use"))
		return
	}
	url, err := ms.store.Query(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

func (ms *MainServer) shorten(w http.ResponseWriter, r *http.Request) {
	resp := util.SetShortenQueryResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	urlStr := r.Form.Get("url")
	key, err := ms.store.Store(urlStr)
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
		resp.Key = key
		resp.OriginalURL = urlStr
	}
	util.WriteJSON(w, resp)
}

func (ms *MainServer) setReserve(w http.ResponseWriter, r *http.Request) {
	resp := util.SetShortenQueryResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	resp.Key = r.Form.Get("key")
	urlStr := r.Form.Get("url")
	err = ms.store.SetReserve(resp.Key, urlStr)
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
		resp.OriginalURL = urlStr
	}
	util.WriteJSON(w, resp)
}

func (ms *MainServer) reserve(w http.ResponseWriter, r *http.Request) {
	resp := util.ReserveResponse{}
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
	resp.Keys, err = ms.store.Reserve(int(num))
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
	}
	util.WriteJSON(w, resp)
}

func (ms *MainServer) query(w http.ResponseWriter, r *http.Request) {
	resp := util.SetShortenQueryResponse{}
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	resp.Key = r.Form.Get("key")
	url, err := ms.store.Query(resp.Key)
	if err != nil {
		resp.Succeeded = false
		resp.ErrorMsg = err.Error()
	} else {
		resp.Succeeded = true
		resp.OriginalURL = url
	}
	util.WriteJSON(w, resp)
}
