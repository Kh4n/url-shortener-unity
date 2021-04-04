package shortener

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func SimplePostForwarder(target string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid target URL")
	}
	ret := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.Host = u.Host
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.URL.Path = SingleJoiningSlash(u.Path, req.URL.Path)
		},
	}
	return ret, nil
}

func CheckUrl(urlStr string) error {
	_, err := http.Get(urlStr)
	if err != nil {
		return err
	}
	return nil
}

func CheckAll(urls []string) error {
	for _, u := range urls {
		err := CheckUrl(u)
		if err != nil {
			return err
		}
	}
	return nil
}

func SingleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		return a + "/" + b
	}
	return a + b
}
