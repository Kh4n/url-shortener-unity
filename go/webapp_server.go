package shortener

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
)

// simple server that either forwards requests to a backend
// or queries the backend and redirects users to full URL
type WebappServer struct {
	mux    *http.ServeMux
	client *http.Client
	home   http.Handler

	backendServer string
}

func NewWebappServer(webDir, backendServerHost string) (*WebappServer, error) {
	ret := &WebappServer{
		mux:    http.NewServeMux(),
		client: &http.Client{},
		home:   http.FileServer(http.Dir(webDir)),

		backendServer: fmt.Sprintf("http://%s", backendServerHost),
	}
	proxy, err := SimplePostForwarder(ret.backendServer)
	if err != nil {
		return nil, fmt.Errorf("error creating webapp server: %s", err.Error())
	}

	ret.mux.HandleFunc("/", ret.redirect)
	ret.mux.HandleFunc(SHORTEN_ENDPOINT, proxy.ServeHTTP)

	err = CheckUrl(ret.backendServer)
	if err != nil {
		return nil, fmt.Errorf("could not connect to backend server: %s", err.Error())
	}

	return ret, nil
}

func (ws *WebappServer) Start(port uint) error {
	log.Printf("Starting webapp server on :%d\n", port)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), ws.mux)
}

func (ws *WebappServer) redirect(w http.ResponseWriter, r *http.Request) {
	// take off leading forward slash
	key := r.URL.Path[1:]
	// if the key isn't valid, just file serve it. will 404 appropriately
	if !ValidKey(key) {
		ws.home.ServeHTTP(w, r)
		return
	}
	jsonResp, _, err := PostSetShortenQuery(
		ws.client, SingleJoiningSlash(ws.backendServer, QUERY_ENDPOINT),
		url.Values{"key": {key}},
	)
	if err != nil {
		log.Printf("Internal server error parsing response: %s\n", err.Error())
		http.Error(w, "Internal server error parsing response", http.StatusInternalServerError)
		return
	}
	if jsonResp.Succeeded {
		http.Redirect(w, r, jsonResp.OriginalURL, REDIRECT_STATUS)
	} else {
		http.NotFound(w, r)
	}
}
