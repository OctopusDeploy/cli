package testutil

import (
	"net/http"
	"testing"
)

// This file contains utilities to help with unit and integration testing

// AssertSuccess checks that err is nil and returns true.
// If it's not, it will print all the args, then write the Error string, fail the test, and return false
func AssertSuccess(t *testing.T, err error, args ...any) bool {
	if err != nil {
		for _, arg := range args {
			t.Log(arg)
		}
		t.Errorf(err.Error())
		return false
	}
	return true
}

func RequireSuccess(t *testing.T, err error, args ...any) bool {
	if err != nil {
		for _, arg := range args {
			t.Log(arg)
		}
		t.Fatalf(err.Error())
		return false
	}
	return true
}

type RoundTripper func(r *http.Request) (*http.Response, error)

func (s RoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	return s(r)
}

// NewMockHttpClient returns an Http Client which returns 200 OK with no response body for everything
func NewMockHttpClient() *http.Client {
	return NewMockHttpClientWithTransport(RoundTripper(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       nil,
		}, nil
	}))
}

func NewMockHttpClientWithTransport(transport http.RoundTripper) *http.Client {
	httpClient := &http.Client{}
	httpClient.Transport = transport
	return httpClient
}
