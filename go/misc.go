package shortener

import "net/http"

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
