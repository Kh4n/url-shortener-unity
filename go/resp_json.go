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
	raw, err := json.Marshal(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	WriteRawJSON(w, raw)
}

func WriteRawJSON(w http.ResponseWriter, raw []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
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

func PostSetShortenQuery(client *http.Client, addr string, args url.Values) (SetShortenQueryResponse, []byte, error) {
	body, err := ReadPost(client, addr, args)
	if err != nil {
		return SetShortenQueryResponse{}, nil, err
	}
	var jsonResp SetShortenQueryResponse
	err = json.Unmarshal(body, &jsonResp)
	if err != nil {
		return SetShortenQueryResponse{}, nil, err
	}
	return jsonResp, body, nil
}
