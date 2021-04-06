package shortener

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

var (
	ErrEmptyStack = errors.New("Pop from empty stack")
)

const (
	KEY_404    uint32 = 0
	KEY_EXISTS uint32 = 1

	KEY_404_EXPIRE = 10 // seconds
)

type cacheKey struct {
	key    string
	expiry int64
}

//KeyStack: thread safe stack
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

	dbServer   string
	reserveAmt uint32
	ks         KeyStack
}

func NewCacheServer(memcachedHost, dbServerHost string, reserveAmt uint32) (*CacheServer, error) {
	ret := &CacheServer{
		mc:     memcache.New(memcachedHost),
		client: &http.Client{},
		mux:    http.NewServeMux(),

		reserveAmt: reserveAmt,
		dbServer:   fmt.Sprintf("http://%s", dbServerHost),
	}
	ret.mux.HandleFunc(QUERY_ENDPOINT, ret.query)
	ret.mux.HandleFunc(SHORTEN_ENDPOINT, ret.shorten)

	err := CheckAll([]string{
		ret.dbServer,
		SingleJoiningSlash(ret.dbServer, SHORTEN_ENDPOINT),
		SingleJoiningSlash(ret.dbServer, QUERY_ENDPOINT),
		SingleJoiningSlash(ret.dbServer, RESERVE_ENDPOINT),
		SingleJoiningSlash(ret.dbServer, SETRESERVE_ENDPOINT),
	})
	if err != nil {
		return nil, fmt.Errorf("unable to connect to main server: %s", err)
	}
	err = nil
	for i := 0; i < 10; i++ {
		err = ret.mc.Ping()
		if err == nil {
			break
		}
		log.Printf("Unable to connect to cache server, retrying in 1s")
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to connect to memcached server: %s", err)
	}
	return ret, nil
}

func (cs *CacheServer) Start(port uint) error {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		err := cs.Close()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}()
	log.Printf("Starting cache server on :%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), cs.mux)
}

func (cs *CacheServer) Close() error {
	log.Println("Closed cache server successfully")
	return nil
}

func (cs *CacheServer) query(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("400 - Bad Request"))
		return
	}
	key := r.Form.Get("key")
	// first check our cache
	urlIt, err := cs.mc.Get(key)
	if err == nil {
		WriteRawJSON(w, urlIt.Value)
		return
	}
	// query the main server if we have a cache miss
	jsonResp, raw, err := PostSetShortenQuery(
		cs.client, SingleJoiningSlash(cs.dbServer, QUERY_ENDPOINT),
		url.Values{"key": {key}},
	)
	if err != nil {
		log.Printf("Internal server error parsing response: %s\n", err.Error())
		http.Error(w, "Internal server error parsing response", http.StatusInternalServerError)
		return
	}
	err = cs.cacheResp(raw, &jsonResp)
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
	// validate url early as opposed to asking main server to validate it
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
	// can't use expired keys as the main server has reclaimed them
	if err == nil && key.expiry > time.Now().Unix() {
		// we still need to update the main server, but that can be done
		// asynchronously. this means that other cache servers will not
		// immediately experience the changes until the main server receives this request
		go func() {
			jsonResp, err := cs.setReserve(key.key, urlStr)
			if err != nil {
				log.Printf("Internal server error pushing shorten: %s\n", err.Error())
			} else if !jsonResp.Succeeded {
				log.Printf("Internal server error pushing shorten: %s\n", jsonResp.ErrorMsg)
			}
		}()
		resp := SetShortenQueryResponse{
			Succeeded:   true,
			Key:         key.key,
			OriginalURL: urlStr,
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Internal server error marshalling response: %s\n", err.Error())
			http.Error(w, "Internal server error marshalling response", http.StatusInternalServerError)
		}
		err = cs.cacheResp(raw, &resp)
		if err != nil {
			log.Printf("Internal server error caching key: %s\n", err.Error())
			http.Error(w, "Internal server error caching key", http.StatusInternalServerError)
			return
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
	// update the main server, synchronously this time as we have to wait
	// for a response in order to serve the request
	jsonResp, raw, err := cs.pushShorten(urlStr)
	if err != nil {
		log.Printf("Internal server error pushing shorten: %s\n", err.Error())
		http.Error(w, "Internal server error pushing shorten", http.StatusInternalServerError)
		return
	}
	cs.cacheResp(raw, &jsonResp)
	WriteJSON(w, jsonResp)
}

func (cs *CacheServer) reserveKeys() error {
	body, err := ReadPost(
		cs.client, SingleJoiningSlash(cs.dbServer, RESERVE_ENDPOINT),
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
			// only hold keys for 16 hours
			jsonResp.Keys[i], time.Now().Add(CACHE_RESERVE_EXPIRY).Unix(),
		})
	}
	cs.ks.PushAll(newKeys)
	return nil
}

func (cs *CacheServer) pushShorten(urlStr string) (SetShortenQueryResponse, []byte, error) {
	jsonResp, raw, err := PostSetShortenQuery(
		cs.client, SingleJoiningSlash(cs.dbServer, SHORTEN_ENDPOINT),
		url.Values{"url": {urlStr}},
	)
	if err != nil {
		return SetShortenQueryResponse{}, nil, err
	}
	return jsonResp, raw, nil
}

func (cs *CacheServer) setReserve(key, urlStr string) (SetShortenQueryResponse, error) {
	jsonResp, _, err := PostSetShortenQuery(
		cs.client, SingleJoiningSlash(cs.dbServer, SETRESERVE_ENDPOINT),
		url.Values{"key": {key}, "url": {urlStr}},
	)
	if err != nil {
		return SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}

func (cs *CacheServer) cacheResp(raw []byte, jsonResp *SetShortenQueryResponse) error {
	if jsonResp.Succeeded {
		return cs.mc.Set(&memcache.Item{
			Key: jsonResp.Key, Value: raw,
			Expiration: 0,
		})
	}
	return cs.mc.Set(&memcache.Item{
		Key: jsonResp.Key, Value: raw,
		Expiration: KEY_404_EXPIRE,
	})
}
