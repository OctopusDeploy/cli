package testutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"io/ioutil"
	"net/http"
)

type FakeRequestHandler struct {
	// Method we expect the client to be using (GET, POST etc)
	Method string
	// Path we expect the client to be interacting with (note for http://server/api?foo this is just api?foo
	PathAndQuery string

	// Handler function to call which generates the response, should the Method+UrlString match
	Handler func(r *http.Request) (*http.Response, error)
}

type FakeApiResponder struct {
	responderQueue []FakeRequestHandler
}

// conforms to RoundTripper so we can use it for httpClient.Transport

func (f *FakeApiResponder) RoundTrip(r *http.Request) (*http.Response, error) {
	handler := f.tryDequeueHandler()
	if handler == nil {
		return nil, errors.New(fmt.Sprintf("Test Failure! No responder for %s %s", r.Method, r.URL))
	}

	pathAndQuery := r.URL.Path
	if r.URL.RawQuery != "" {
		pathAndQuery = fmt.Sprintf("%s?%s", pathAndQuery, r.URL.RawQuery)
	}
	if handler.Method != r.Method || handler.PathAndQuery != pathAndQuery {
		return nil, errors.New(fmt.Sprintf("Test Failure! Client sent %s %s, have responder for %s %s", r.Method, pathAndQuery, handler.Method, handler.PathAndQuery))
	}

	// if we get here we are good to go
	return handler.Handler(r)
}

// higher level helper where your lambda can just return 'any' and it will get serialised into JSON and
// sent as a response
func (f *FakeApiResponder) EnqueueResponder(method string, pathAndQuery string, handler func(r *http.Request) (any, error)) {
	rawHandler := func(r *http.Request) (*http.Response, error) {
		responseObject, err := handler(r)
		if err != nil {
			return nil, err
		}

		body, err := json.Marshal(responseObject)
		if err != nil {
			return nil, err
		}

		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          ioutil.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
		}, nil
	}

	fh := FakeRequestHandler{
		Method:       method,
		PathAndQuery: pathAndQuery,
		Handler:      rawHandler,
	}
	f.responderQueue = append(f.responderQueue, fh)
}

func (f *FakeApiResponder) EnqueueRawResponder(method string, pathAndQuery string, handler func(r *http.Request) (*http.Response, error)) {
	fh := FakeRequestHandler{
		Method:       method,
		PathAndQuery: pathAndQuery,
		Handler:      handler,
	}
	f.responderQueue = append(f.responderQueue, fh)
}

// returns a pointer to the dequeued handler. Nil if the queue is empty
func (f *FakeApiResponder) tryDequeueHandler() *FakeRequestHandler {
	if f.responderQueue == nil || len(f.responderQueue) == 0 {
		return nil
	}

	head, rest := f.responderQueue[0], f.responderQueue[1:]
	f.responderQueue = rest
	return &head
}

// RemainingQueueLength returns the number of unprocessed responders. You should always
// assert that this is zero at the end of a test to ensure that you haven't succeeded accidentally.
func (f *FakeApiResponder) RemainingQueueLength() int {
	return len(f.responderQueue)
}

func NewFakeApiResponder() *FakeApiResponder {
	return &FakeApiResponder{responderQueue: []FakeRequestHandler{}}
}

// The octopus client library always starts by doing a GET on /api; here's a standard handler for that
func EnqueueRootResponder(fakeServer *FakeApiResponder) {
	fakeServer.EnqueueResponder("GET", "/api", func(r *http.Request) (any, error) {
		return NewRootResource(), nil
	})
}

func NewRootResource() *octopusApiClient.RootResource {
	root := octopusApiClient.NewRootResource()
	root.Links["Spaces"] = "/api/spaces{/id}{?skip,ids,take,partialName}"
	root.Links["Projects"] = "/api/Spaces-1/projects{/id}{?name,skip,ids,clone,take,partialName,clonedFromProjectId}"
	root.Links["Channels"] = "/api/Spaces-1/channels{/id}{?skip,take,ids,partialName}"
	root.Links["DeploymentProcesses"] = "/api/Spaces-1/deploymentprocesses{/id}{?skip,take,ids}"
	return root
}
