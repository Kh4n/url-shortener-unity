package shortener

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

const (
	SHORTEN_ENDPOINT    = "/api/shorten"
	QUERY_ENDPOINT      = "/api/query"
	RESERVE_ENDPOINT    = "/api/reserve"
	SETRESERVE_ENDPOINT = "/api/setReserve"

	REDIRECT_STATUS = http.StatusMovedPermanently
)

type MainServer struct {
	store *URLStore

	port uint32
	mux  *http.ServeMux
	home http.Handler
}

func NewMainServer(dbLocation string, port uint32) (*MainServer, error) {
	ret := &MainServer{
		mux:  http.NewServeMux(),
		port: port,
	}
	var err error
	ret.store, err = NewURLStore(dbLocation)
	if err != nil {
		return nil, err
	}

	ret.mux.HandleFunc("/", ret.redirect)
	ret.mux.HandleFunc(SHORTEN_ENDPOINT, ret.shorten)
	ret.mux.HandleFunc(QUERY_ENDPOINT, ret.query)

	ret.mux.HandleFunc(RESERVE_ENDPOINT, ret.reserve)
	ret.mux.HandleFunc(SETRESERVE_ENDPOINT, ret.setReserve)

	ret.home = http.FileServer(http.Dir("./web"))

	return ret, nil
}

func (ms *MainServer) Close() error {
	return ms.store.Close()
}

func (ms *MainServer) Start() error {
	log.Println("Starting server on :8080")
	return http.ListenAndServe(fmt.Sprintf(":%d", ms.port), ms.mux)
}

func (ms *MainServer) redirect(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	if !ValidKey(key) {
		ms.home.ServeHTTP(w, r)
		return
	}
	url, err := ms.store.Query(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, url, REDIRECT_STATUS)
}

func (ms *MainServer) shorten(w http.ResponseWriter, r *http.Request) {
	resp := SetShortenQueryResponse{}
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
	WriteJSON(w, resp)
}

func (ms *MainServer) setReserve(w http.ResponseWriter, r *http.Request) {
	resp := SetShortenQueryResponse{}
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
	WriteJSON(w, resp)
}

func (ms *MainServer) reserve(w http.ResponseWriter, r *http.Request) {
	resp := ReserveResponse{}
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
	WriteJSON(w, resp)
}

func (ms *MainServer) query(w http.ResponseWriter, r *http.Request) {
	resp := SetShortenQueryResponse{}
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
	WriteJSON(w, resp)
}
