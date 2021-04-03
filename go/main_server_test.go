package shortener

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func GetRequest(target string, args url.Values) *http.Request {
	ret := httptest.NewRequest("GET", target, strings.NewReader(args.Encode()))
	ret.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return ret
}

func PostRequest(target string, args url.Values) *http.Request {
	ret := httptest.NewRequest("POST", target, strings.NewReader(args.Encode()))
	ret.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return ret
}

func RecordGet(mux *http.ServeMux, target string, args url.Values) *httptest.ResponseRecorder {
	req := GetRequest(target, args)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func HttpTestPostSetQueryShorten(
	mux *http.ServeMux, target string, args url.Values) (SetShortenQueryResponse, *httptest.ResponseRecorder, error) {
	req := PostRequest(
		target, args,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		return SetShortenQueryResponse{}, nil, err
	}
	var jsonResp SetShortenQueryResponse
	json.Unmarshal(body, &jsonResp)
	return jsonResp, rec, nil
}

func HttpTestPostReserve(
	mux *http.ServeMux, target string, args url.Values) (ReserveResponse, *httptest.ResponseRecorder, error) {
	req := PostRequest(
		target, args,
	)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	body, err := ioutil.ReadAll(rec.Body)
	if err != nil {
		return ReserveResponse{}, nil, err
	}
	var jsonResp ReserveResponse
	json.Unmarshal(body, &jsonResp)
	return jsonResp, rec, nil
}

func CheckJSONResponse(t *testing.T, jsonResp interface{}, target interface{}) {
	if !reflect.DeepEqual(jsonResp, target) {
		t.Errorf("Expected response %+v, got response %+v", target, jsonResp)
	}
}

func TestMainServerBasic(t *testing.T) {
	testDB := "./test_db"
	server, err := NewMainServer(testDB, 0)
	if err != nil {
		t.Errorf("Unable to create test server: %s", err.Error())
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			exampleUrl := "http://example.com"
			jsonResp, rec, err := HttpTestPostSetQueryShorten(
				server.mux, SHORTEN_ENDPOINT, url.Values{"url": {exampleUrl}},
			)
			if err != nil {
				t.Errorf("Unable to mock post: %s", err.Error())
			}
			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("Expected 200 ok, got : %d", rec.Result().StatusCode)
			}
			CheckJSONResponse(t, &jsonResp, &SetShortenQueryResponse{
				Succeeded:   true,
				Key:         jsonResp.Key,
				OriginalURL: exampleUrl,
			})

			key := jsonResp.Key
			jsonResp, rec, err = HttpTestPostSetQueryShorten(
				server.mux, QUERY_ENDPOINT, url.Values{"key": {key}},
			)
			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("Expected 200 ok, got : %d", rec.Result().StatusCode)
			}
			if err != nil {
				t.Errorf("Unable to mock post: %s", err.Error())
			}
			CheckJSONResponse(t, &jsonResp, &SetShortenQueryResponse{
				Succeeded:   true,
				Key:         key,
				OriginalURL: exampleUrl,
			})

			jsonResp, rec, err = HttpTestPostSetQueryShorten(
				server.mux, QUERY_ENDPOINT, url.Values{"key": {"BADKEY"}},
			)
			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("Expected 200 ok, got : %d", rec.Result().StatusCode)
			}
			if err != nil {
				t.Errorf("Unable to mock post: %s", err.Error())
			}
			CheckJSONResponse(t, &jsonResp, &SetShortenQueryResponse{
				Succeeded: false,
				ErrorMsg:  jsonResp.ErrorMsg,
				Key:       "BADKEY",
			})

			var reserveResp ReserveResponse
			reserveResp, rec, err = HttpTestPostReserve(
				server.mux, RESERVE_ENDPOINT, url.Values{"num": {fmt.Sprintf("%d", 5)}},
			)
			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("Expected 200 ok, got : %d", rec.Result().StatusCode)
			}
			if err != nil {
				t.Errorf("Unable to mock post: %s", err.Error())
			}
			CheckJSONResponse(t, &reserveResp, &ReserveResponse{
				Succeeded: true,
				Keys:      reserveResp.Keys,
			})

			key = reserveResp.Keys[4]
			jsonResp, rec, err = HttpTestPostSetQueryShorten(
				server.mux, SETRESERVE_ENDPOINT, url.Values{"key": {key}, "url": {exampleUrl}},
			)
			if rec.Result().StatusCode != http.StatusOK {
				t.Errorf("Expected 200 ok, got : %d", rec.Result().StatusCode)
			}
			if err != nil {
				t.Errorf("Unable to mock post: %s", err.Error())
			}
			CheckJSONResponse(t, &jsonResp, &SetShortenQueryResponse{
				Succeeded:   true,
				Key:         key,
				OriginalURL: exampleUrl,
			})

			rec = RecordGet(server.mux, "/"+key, url.Values{})
			if rec.Result().StatusCode != REDIRECT_STATUS {
				t.Errorf("Expected redirect status %d, got %d", REDIRECT_STATUS, rec.Result().StatusCode)
			}
			rec = RecordGet(server.mux, "/BADKEY", url.Values{})
			if rec.Result().StatusCode != http.StatusNotFound {
				t.Errorf("Expected redirect status not found, got %d", rec.Result().StatusCode)
			}
			wg.Done()
		}()
	}
	wg.Wait()
	t.Cleanup(func() {
		err := server.Close()
		if err != nil {
			t.Errorf("Unable to close server: %s", err.Error())
		}
		err = os.RemoveAll(testDB)
		if err != nil {
			t.Fatalf("Could not remove test db directory: %s", err.Error())
		}
	})
}
