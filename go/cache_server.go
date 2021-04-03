package shortener

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

var (
	ErrEmptyStack = errors.New("Pop from empty stack")
)

const (
	KEY_404    uint32 = 0
	KEY_EXISTS uint32 = 1

	KEY_404_EXPIRE = 60 // seconds
)

type cacheKey struct {
	key    string
	expiry int64
}

type KeyStack struct {
	stack []cacheKey
	lock  sync.Mutex
}

func (ks *KeyStack) PushAll(vals []cacheKey) {
	ks.lock.Lock()
	ks.stack = append(ks.stack, vals...)
	ks.lock.Unlock()
}

func (ks *KeyStack) Pop() (cacheKey, error) {
	ks.lock.Lock()
	if len(ks.stack) == 0 {
		ks.lock.Unlock()
		return cacheKey{}, ErrEmptyStack
	}
	ret := ks.stack[len(ks.stack)-1]
	ks.stack = ks.stack[:len(ks.stack)-1]
	ks.lock.Unlock()
	return ret, nil
}

type CacheServer struct {
	mc     *memcache.Client
	mux    *http.ServeMux
	client *http.Client
	home   http.Handler

	port       uint32
	mainServer string
	reserveAmt uint32
	ks         KeyStack
}

func NewCacheServer(memcacheAddr, mainServerAddr string, port, reserveAmt uint32) (*CacheServer, error) {
	ret := &CacheServer{
		mc:     memcache.New(memcacheAddr),
		client: &http.Client{},
		mux:    http.NewServeMux(),

		port:       port,
		reserveAmt: reserveAmt,
		mainServer: mainServerAddr,
	}
	ret.mux.HandleFunc("/", ret.redirect)
	ret.mux.HandleFunc(SHORTEN_ENDPOINT, ret.shorten)

	ret.home = http.FileServer(http.Dir("./web"))

	err := CheckAll([]string{
		mainServerAddr,
		mainServerAddr + SHORTEN_ENDPOINT,
		mainServerAddr + QUERY_ENDPOINT,
		mainServerAddr + RESERVE_ENDPOINT,
		mainServerAddr + SETRESERVE_ENDPOINT,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to main server: %s", err)
	}
	return ret, nil
}

func (cs *CacheServer) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", cs.port), cs.mux)
}

func (cs *CacheServer) redirect(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path[1:]
	if !ValidKey(key) {
		cs.home.ServeHTTP(w, r)
		return
	}
	urlIt, err := cs.mc.Get(key)
	if err == nil {
		if urlIt.Flags == KEY_EXISTS {
			http.Redirect(w, r, string(urlIt.Value), REDIRECT_STATUS)
		} else {
			http.NotFound(w, r)
		}
		return
	}
	jsonResp, err := PostSetShortenQuery(cs.client, cs.mainServer+QUERY_ENDPOINT, url.Values{"key": {key}})
	if err != nil {
		log.Printf("Internal server error parsing response: %s\n", err.Error())
		http.Error(w, "Internal server error parsing response", http.StatusInternalServerError)
		return
	}
	err = cs.cacheResp(&jsonResp)
	if err != nil {
		log.Printf("Internal server error caching response: %s\n", err.Error())
		http.Error(w, "Internal server error caching response", http.StatusInternalServerError)
		return
	}
	if jsonResp.Succeeded {
		http.Redirect(w, r, jsonResp.OriginalURL, REDIRECT_STATUS)
	} else {
		http.NotFound(w, r)
	}
}

func (cs *CacheServer) shorten(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}

	urlStr := r.Form.Get("url")
	if !ValidUrl(urlStr) {
		resp := SetShortenQueryResponse{
			Succeeded:   false,
			Key:         "",
			OriginalURL: urlStr,
			ErrorMsg:    fmt.Sprintf("Invalid url: %s", urlStr),
		}
		WriteJSON(w, resp)
		return
	}
	key, err := cs.ks.Pop()
	if err == nil && key.expiry > time.Now().Unix() {
		go func() {
			jsonResp, err := cs.setReserve(key.key, urlStr)
			if err != nil {
				log.Printf("Internal server error pushing shorten: %s\n", err.Error())
			} else if !jsonResp.Succeeded {
				log.Printf("Internal server error pushing shorten: %s\n", jsonResp.ErrorMsg)
			}
		}()
		err = cs.cacheKey(key.key, urlStr, KEY_EXISTS)
		if err != nil {
			log.Printf("Internal server error caching key: %s\n", err.Error())
			http.Error(w, "Internal server error caching key", http.StatusInternalServerError)
			return
		}
		resp := SetShortenQueryResponse{
			Succeeded:   true,
			Key:         key.key,
			OriginalURL: urlStr,
		}
		WriteJSON(w, resp)
		return
	}
	go func() {
		err := cs.reserveKeys()
		if err != nil {
			log.Printf("Internal server error: %s\n", err.Error())
		}
	}()
	jsonResp, err := cs.pushShorten(urlStr)
	if err != nil {
		log.Printf("Internal server error pushing shorten: %s\n", err.Error())
		http.Error(w, "Internal server error pushing shorten", http.StatusInternalServerError)
		return
	}
	if jsonResp.Succeeded {
		cs.cacheResp(&jsonResp)
	}
	WriteJSON(w, jsonResp)
}

func (cs *CacheServer) reserveKeys() error {
	body, err := ReadPost(
		cs.client, cs.mainServer+RESERVE_ENDPOINT,
		url.Values{"num": {fmt.Sprintf("%d", cs.reserveAmt)}},
	)
	if err != nil {
		return err
	}
	var jsonResp ReserveResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return err
	}
	if !jsonResp.Succeeded {
		return errors.New(jsonResp.ErrorMsg)
	}
	newKeys := make([]cacheKey, 0, len(jsonResp.Keys))
	for i := 0; i < len(jsonResp.Keys); i++ {
		newKeys = append(newKeys, cacheKey{
			jsonResp.Keys[i], time.Now().Add(CACHE_RESERVE_EXPIRY).Unix(),
		})
	}
	cs.ks.PushAll(newKeys)
	return nil
}

func (cs *CacheServer) pushShorten(urlStr string) (SetShortenQueryResponse, error) {
	jsonResp, err := PostSetShortenQuery(
		cs.client, cs.mainServer+SHORTEN_ENDPOINT,
		url.Values{"url": {urlStr}},
	)
	if err != nil {
		return SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}

func (cs *CacheServer) setReserve(key, urlStr string) (SetShortenQueryResponse, error) {
	jsonResp, err := PostSetShortenQuery(
		cs.client, cs.mainServer+SETRESERVE_ENDPOINT,
		url.Values{"key": {key}, "url": {urlStr}},
	)
	if err != nil {
		return SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}

func (cs *CacheServer) cacheResp(jsonResp *SetShortenQueryResponse) error {
	if jsonResp.Succeeded {
		return cs.cacheKey(jsonResp.Key, jsonResp.OriginalURL, KEY_EXISTS)
	}
	return cs.cacheKey(jsonResp.Key, jsonResp.OriginalURL, KEY_404)
}

func (cs *CacheServer) cacheKey(key, urlStr string, keyResponse uint32) error {
	var expire int32 = 0
	if keyResponse == KEY_404 {
		expire = KEY_404_EXPIRE
	}
	err := cs.mc.Set(&memcache.Item{
		Key: key, Value: []byte(urlStr),
		Flags: keyResponse, Expiration: expire,
	})
	return err
}
