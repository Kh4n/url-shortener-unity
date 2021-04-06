package shortener

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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
	mux   *http.ServeMux
}

func NewMainServer(dbLocation string) (*MainServer, error) {
	ret := &MainServer{
		mux: http.NewServeMux(),
	}
	var err error
	ret.store, err = NewURLStore(dbLocation)
	if err != nil {
		return nil, err
	}

	ret.mux.HandleFunc(QUERY_ENDPOINT, ret.query)
	ret.mux.HandleFunc(SHORTEN_ENDPOINT, ret.shorten)

	ret.mux.HandleFunc(RESERVE_ENDPOINT, ret.reserve)
	ret.mux.HandleFunc(SETRESERVE_ENDPOINT, ret.setReserve)

	return ret, nil
}

func (ms *MainServer) Close() error {
	err := ms.store.Close()
	if err != nil {
		log.Printf("Error closing server: %s\n", err.Error())
		return err
	}
	log.Println("Closed dbserver successfully")
	return nil
}

func (ms *MainServer) Start(port uint) error {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		err := ms.Close()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}()
	log.Printf("Starting db server on :%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), ms.mux)
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

	num, err := strconv.ParseUint(r.Form.Get("num"), 10, MAX_RESERVE_NUM_BITS)
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
