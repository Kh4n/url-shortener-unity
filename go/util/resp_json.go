package util

import (
	"encoding/json"
	"net/http"
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
