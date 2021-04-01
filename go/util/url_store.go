package util

import (
	"math/rand"
	"net/url"

	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
)

// go does not support constant arrays. this should never be modified
// we also cannot afford to make a function to create and return this
// LUT over and over again :/
var base62Lut = []byte{
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j',
	'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't',
	'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D',
	'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N',
	'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X',
	'Y', 'Z',
}

const MAX_URL_LEN = 1 << 13
const MAX_RESERVE_NUM = 1 << 14

// encodes ret with num coverted to a base62 number in reverse
// ret is cleared before use
func base62Encode(num uint64, ret *[]byte) {
	*ret = (*ret)[:0]
	for num >= 62 {
		*ret = append(*ret, base62Lut[num%62])
		num /= 62
	}
	*ret = append(*ret, base62Lut[num])
}

type URLStore struct {
	db *badger.DB
}

func NewURLStore(path string) (ret *URLStore, err error) {
	ret = new(URLStore)
	ret.db, err = badger.Open(badger.DefaultOptions(path).WithTruncate(true))
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Store stores the url in DB, returning the created key
func (store *URLStore) Store(urlStr string) (string, error) {
	if !validateUrl(urlStr) {
		return "", errors.Errorf("Invalid url: %s", urlStr)
	}
	key := make([]byte, 0, 7)
	err := store.db.Update(func(txn *badger.Txn) error {
		base62Encode(rand.Uint64(), &key)
		for _, err := txn.Get(key); err != badger.ErrKeyNotFound; {
			base62Encode(rand.Uint64(), &key)
		}
		err := txn.Set(key, []byte(urlStr))
		return err
	})
	if err != nil {
		return "", err
	}
	return string(key), nil
}

// Queries a key for a URL
func (store *URLStore) Query(key string) (string, error) {
	var ret string
	err := store.db.View(func(txn *badger.Txn) error {
		v, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		err = v.Value(func(val []byte) error {
			ret = string(val)
			return nil
		})
		if err != nil || len(ret) == 0 {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return ret, err
}

// Reserves num keys and returns them. These keys can then be handed
// to a cache server so that the cache server can serve both create
// and query requests
func (store *URLStore) Reserve(num int) ([]string, error) {
	if num <= 0 || num >= MAX_RESERVE_NUM {
		return []string{}, errors.Errorf("Invalid num %d\n", num)
	}
	ret := make([]string, 0, num)
	err := store.db.Update(func(txn *badger.Txn) error {
		key := make([]byte, 0, 7)
		for i := 0; i < num; i++ {
			base62Encode(rand.Uint64(), &key)
			for _, err := txn.Get(key); err != badger.ErrKeyNotFound; {
				base62Encode(rand.Uint64(), &key)
			}
			err := txn.Set(key, []byte(""))
			if err != nil {
				return err
			}
			ret = append(ret, string(key))
		}
		return nil
	})
	if err != nil {
		return []string{}, err
	}
	return ret, nil
}

// Ensures that the keys have no '?' etc. that would interfere with
// url shortener functionality
func validateKey(key string) bool {
	for _, c := range key {
		if !(c >= '0' && c <= '9' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

func validateUrl(urlStr string) bool {
	if len(urlStr) > MAX_URL_LEN {
		return false
	}
	u, err := url.Parse(urlStr)
	return err == nil && u.Scheme != "" && u.Host != ""
}

// Set sets a shortened url key to the url, if the key is not in
// use. This is primarily for cache servers to use with their reserved keys
func (store *URLStore) Set(key string, urlStr string) error {
	if !validateKey(key) {
		return errors.Errorf("Invalid key: %s", key)
	}
	if !validateUrl(urlStr) {
		return errors.Errorf("Invalid url: %s", urlStr)
	}
	keyBytes := []byte(key)
	err := store.db.Update(func(txn *badger.Txn) error {
		if v, err := txn.Get(keyBytes); !(err == badger.ErrKeyNotFound || v.ValueSize() == 0) {
			return errors.Errorf("Attempted to set key that already exists: %s", key)
		}
		err := txn.Set(keyBytes, []byte(urlStr))
		return err
	})
	if err != nil {
		return err
	}
	return nil
}
