package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/Kh4n/url-shortener-unity/go/util"
	"github.com/bradfitz/gomemcache/memcache"
)

var (
	ErrEmptyStack = errors.New("Pop from empty stack")
)

const (
	KEY_404    uint32 = 0
	KEY_EXISTS uint32 = 1

	KEY_404_EXPIRE = 60
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
	client *http.Client
	mux    *http.ServeMux

	port       uint32
	mainServer string
	reserveAmt uint32
	ks         KeyStack
}

func NewCacheServer(memcacheAddr, mainServerAddr string, port, reserveAmt uint32) *CacheServer {
	ret := &CacheServer{
		mc:     memcache.New(memcacheAddr),
		client: &http.Client{},
		mux:    http.NewServeMux(),

		port:       port,
		reserveAmt: reserveAmt,
		mainServer: mainServerAddr,
	}
	ret.mux.HandleFunc("/", ret.redirect)
	ret.mux.HandleFunc("/shorten", ret.shorten)
	return ret
}

func (cs *CacheServer) Start() error {
	return http.ListenAndServe(fmt.Sprintf(":%d", cs.port), cs.mux)
}

func (cs *CacheServer) redirect(w http.ResponseWriter, r *http.Request) {
	key := path.Base(r.URL.Path)
	if key == "/" {
		w.Write([]byte("create a post request to /shorten to use"))
		return
	}
	urlIt, err := cs.mc.Get(key)
	if err == nil {
		if urlIt.Flags == 1 {
			http.Redirect(w, r, string(urlIt.Value), http.StatusFound)
		} else {
			http.NotFound(w, r)
		}
		return
	}
	resp, err := cs.client.PostForm(cs.mainServer+"/query", url.Values{"key": {key}})
	if err != nil {
		log.Printf("Internal server error sending request: %s\n", err.Error())
		http.Error(w, "Internal server error sending request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Internal server error parsing server response: %s\n", err.Error())
		http.Error(w, "Internal server error parsing server response", http.StatusInternalServerError)
		return
	}
	var jsonResp util.SetShortenQueryResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		log.Printf("Internal server error parsing JSON: %s\n", err.Error())
		http.Error(w, "Internal server error parsing JSON", http.StatusInternalServerError)
		return
	}
	err = cs.cacheResp(&jsonResp)
	if err != nil {
		log.Printf("Internal server error caching response: %s\n", err.Error())
		http.Error(w, "Internal server error caching response", http.StatusInternalServerError)
		return
	}
	if jsonResp.Succeeded {
		http.Redirect(w, r, jsonResp.OriginalURL, http.StatusFound)
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
	key, err := cs.ks.Pop()
	if err == nil && key.expiry > time.Now().Unix() {
		go func() {
			jsonResp, err := cs.setReserve(key.key, urlStr)
			if err != nil || !jsonResp.Succeeded {
				log.Printf("Internal server error pushing shorten: %s %s\n", err.Error(), jsonResp.ErrorMsg)
			}
		}()
		err = cs.cacheKey(key.key, urlStr, KEY_EXISTS)
		if err != nil {
			log.Printf("Internal server error caching key: %s\n", err.Error())
			http.Error(w, "Internal server error caching key", http.StatusInternalServerError)
			return
		}
		resp := util.SetShortenQueryResponse{
			Succeeded:   true,
			Key:         key.key,
			OriginalURL: urlStr,
		}
		util.WriteJSON(w, resp)
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
	util.WriteJSON(w, jsonResp)
}

func (cs *CacheServer) reserveKeys() error {
	resp, err := cs.client.PostForm(
		cs.mainServer+"/reserve",
		url.Values{"num": {fmt.Sprintf("%d", cs.reserveAmt)}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var jsonResp util.ReserveResponse
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
			jsonResp.Keys[i], time.Now().Add(util.CACHE_RESERVE_EXPIRY).Unix(),
		})
	}
	cs.ks.PushAll(newKeys)
	return nil
}

func (cs *CacheServer) pushShorten(urlStr string) (util.SetShortenQueryResponse, error) {
	resp, err := cs.client.PostForm(cs.mainServer+"/shorten", url.Values{"url": {urlStr}})
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	var jsonResp util.SetShortenQueryResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}

func (cs *CacheServer) setReserve(key, urlStr string) (util.SetShortenQueryResponse, error) {
	resp, err := cs.client.PostForm(cs.mainServer+"/setReserve", url.Values{"key": {key}, "url": {urlStr}})
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	var jsonResp util.SetShortenQueryResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return util.SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}

func (cs *CacheServer) cacheResp(jsonResp *util.SetShortenQueryResponse) error {
	if jsonResp.Succeeded {
		return cs.cacheKey(jsonResp.Key, jsonResp.OriginalURL, KEY_EXISTS)
	}
	return cs.cacheKey(jsonResp.Key, jsonResp.OriginalURL, KEY_404)
}

func (cs *CacheServer) cacheKey(key, urlStr string, keyResponse uint32) error {
	var expire int32 = 0
	if keyResponse == KEY_404 {
		expire = 60
	}
	err := cs.mc.Set(&memcache.Item{
		Key: key, Value: []byte(urlStr),
		Flags: keyResponse, Expiration: expire,
	})
	return err
}
