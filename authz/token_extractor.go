package authz

import (
	"errors"
	"net/http"
	"strings"
)

var ErrTokenNotFound = errors.New("token not found")

type TokenExtractor interface {
	ExtractToken(r *http.Request) (string, error)
}

type tokenExtractorFunc func(r *http.Request) (string, error)

var _ TokenExtractor = (tokenExtractorFunc)(nil)

func (f tokenExtractorFunc) ExtractToken(r *http.Request) (string, error) {
	return f(r)
}

func ExtractFromMultipleExtractors(extractors ...TokenExtractor) TokenExtractor {
	return tokenExtractorFunc(func(r *http.Request) (string, error) {
		for _, extractor := range extractors {
			tok, err := extractor.ExtractToken(r)
			if err == nil {
				return tok, nil
			}
		}
		return "", ErrTokenNotFound
	})
}

const authTypeBearer = "Bearer"

func ExtractFromAuthorizationHeader() TokenExtractor {
	return tokenExtractorFunc(func(r *http.Request) (string, error) {
		hv := strings.TrimSpace(r.Header.Get("authorization"))
		v := strings.TrimSpace(strings.TrimPrefix(hv, authTypeBearer))
		if v == "" {
			return "", ErrTokenNotFound
		}
		return v, nil
	})
}

func ExtractFromHeader(name string) TokenExtractor {
	return tokenExtractorFunc(func(r *http.Request) (string, error) {
		v := r.Header.Get(name)
		if v == "" {
			return "", ErrTokenNotFound
		}
		return v, nil
	})
}

func ExtractFromQuery(name string) TokenExtractor {
	return tokenExtractorFunc(func(r *http.Request) (string, error) {
		v := r.URL.Query().Get(name)
		if v == "" {
			return "", ErrTokenNotFound
		}
		return v, nil
	})
}
