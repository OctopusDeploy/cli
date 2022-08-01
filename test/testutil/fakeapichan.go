package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

// ResultWaiter1 is like a cheap version of a Promise that you can't chain (on purpose)
type ResultWaiter1[TResult1 any] struct {
	wg sync.WaitGroup
	r1 TResult1
}

func (w *ResultWaiter1[TResult1]) Wait() TResult1 {
	w.wg.Wait()
	return w.r1
}

func GoBeginFunc1[TResult1 any](action func() TResult1) *ResultWaiter1[TResult1] {
	waiter := &ResultWaiter1[TResult1]{}
	waiter.wg.Add(1)

	go func() {
		defer waiter.wg.Done()
		waiter.r1 = action()
	}()

	return waiter
}

type ResultWaiter2[TResult1 any, TResult2 any] struct {
	wg sync.WaitGroup
	r1 TResult1
	r2 TResult2
}

func (w *ResultWaiter2[TResult1, TResult2]) Wait() (TResult1, TResult2) {
	w.wg.Wait()
	return w.r1, w.r2
}

func GoBeginFunc2[TResult1 any, TResult2 any](action func() (TResult1, TResult2)) *ResultWaiter2[TResult1, TResult2] {
	waiter := &ResultWaiter2[TResult1, TResult2]{}

	waiter.wg.Add(1)

	go func() {
		defer waiter.wg.Done()
		waiter.r1, waiter.r2 = action()
	}()

	return waiter
}

type responseOrError struct {
	response *http.Response
	error    error
}

type MockHttpServer struct {
	// when the client issues a request, we receive it here
	Request chan *http.Request
	// when we want to respond back to the client, we send it here
	Response chan responseOrError

	// so test code can detect unanswered requests or responses at the end.
	// Not strictly neccessary as unanswered req/resp results in a channel deadlock
	// and go panics and kills the process, so we find out about it, but this is a bit
	// less confusing to troubleshoot
	pendingMsgCount int32
}

// conforms to RoundTripper so we can use it for httpClient.Transport

func (m *MockHttpServer) RoundTrip(r *http.Request) (*http.Response, error) {
	// we're the client here, so we send a request down the request channel
	atomic.AddInt32(&m.pendingMsgCount, 1)
	m.Request <- r
	atomic.AddInt32(&m.pendingMsgCount, -1)
	// then we wait for a response via the response channel

	atomic.AddInt32(&m.pendingMsgCount, 1)
	x := <-m.Response
	atomic.AddInt32(&m.pendingMsgCount, -1)
	return x.response, x.error
}

func NewMockHttpServer() *MockHttpServer {
	return &MockHttpServer{
		Request:  make(chan *http.Request),
		Response: make(chan responseOrError),
	}
}

func (m *MockHttpServer) GetPendingMessageCount() int {
	return int(m.pendingMsgCount)
}

func (m *MockHttpServer) ReceiveRequest() *http.Request {
	atomic.AddInt32(&m.pendingMsgCount, 1)
	request := <-m.Request
	atomic.AddInt32(&m.pendingMsgCount, -1)
	return request
}

func (m *MockHttpServer) Respond(response *http.Response, err error) {
	atomic.AddInt32(&m.pendingMsgCount, 1)
	m.Response <- responseOrError{response: response, error: err}
	atomic.AddInt32(&m.pendingMsgCount, -1)
}

// now we build some higher level methods on top of ReceiveRequest

func (m *MockHttpServer) ExpectRequest(t *testing.T, method string, pathAndQuery string) *RequestWrapper {
	r := m.ReceiveRequest()

	rPathAndQuery := r.URL.Path
	if r.URL.RawQuery != "" {
		rPathAndQuery = fmt.Sprintf("%s?%s", rPathAndQuery, r.URL.RawQuery)
	}
	assert.Equal(t, method, r.Method)
	assert.Equal(t, pathAndQuery, rPathAndQuery)

	return &RequestWrapper{r, m}
}

type RequestWrapper struct {
	// in case you need it
	Request *http.Request
	Server  *MockHttpServer
}

func (r *RequestWrapper) RespondWith(responseObject any) {
	if responseObject == nil {
		panic("TODO: implement responses with no body")
	}

	body, _ := json.Marshal(responseObject)

	// Regarding response errors:
	// Note that we would use an error here for a low level thing like a network error.
	// An HTTP error like a 404 or 500 would be considered a valid response with an
	// appropriate status code
	r.Server.Respond(&http.Response{
		StatusCode:    http.StatusOK,
		Body:          ioutil.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}, nil)
}
