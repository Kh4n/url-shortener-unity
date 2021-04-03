package shortener

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
)

type SetShortenQueryResponse struct {
	Succeeded bool   `json:"succeeded"`
	ErrorMsg  string `json:"errorMsg"`

	Key         string `json:"key"`
	OriginalURL string `json:"originalURL"`
}

type ReserveResponse struct {
	Succeeded bool   `json:"succeeded"`
	ErrorMsg  string `json:"errorMsg"`

	Keys []string `json:"keys"`
}

func WriteJSON(w http.ResponseWriter, data interface{}) {
	js, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func ReadPost(client *http.Client, addr string, args url.Values) ([]byte, error) {
	resp, err := client.PostForm(addr, args)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func PostSetShortenQuery(client *http.Client, addr string, args url.Values) (SetShortenQueryResponse, error) {
	body, err := ReadPost(client, addr, args)
	if err != nil {
		return SetShortenQueryResponse{}, err
	}
	var jsonResp SetShortenQueryResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return SetShortenQueryResponse{}, err
	}
	return jsonResp, nil
}
