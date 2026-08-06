package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	octopusApiClient "github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/client"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/constants"
	"github.com/stretchr/testify/assert"
)

func GoBegin[TResult any](action func() TResult) chan TResult {
	c := make(chan TResult)
	go func() {
		c <- action()
	}()
	return c
}

type Pair[T1 any, T2 any] struct {
	Item1 T1
	Item2 T2
}

func GoBegin2[TResult1 any, TResult2 any](action func() (TResult1, TResult2)) chan Pair[TResult1, TResult2] {
	c := make(chan Pair[TResult1, TResult2])
	go func() {
		r1, r2 := action()
		c <- Pair[TResult1, TResult2]{Item1: r1, Item2: r2}
	}()
	return c
}

func ReceivePair[T1 any, T2 any](receiver chan Pair[T1, T2]) (T1, T2) {
	pair := <-receiver
	return pair.Item1, pair.Item2
}

type Triple[T1 any, T2 any, T3 any] struct {
	Item1 T1
	Item2 T2
	Item3 T3
}

func GoBegin3[TResult1 any, TResult2 any, TResult3 any](action func() (TResult1, TResult2, TResult3)) chan Triple[TResult1, TResult2, TResult3] {
	c := make(chan Triple[TResult1, TResult2, TResult3])
	go func() {
		r1, r2, r3 := action()
		c <- Triple[TResult1, TResult2, TResult3]{Item1: r1, Item2: r2, Item3: r3}
	}()
	return c
}

func ReceiveTriple[T1 any, T2 any, T3 any](receiver chan Triple[T1, T2, T3]) (T1, T2, T3) {
	pair := <-receiver
	return pair.Item1, pair.Item2, pair.Item3
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
	// Not strictly necessary as unanswered req/resp results in a channel deadlock
	// and go panics and kills the process, so we find out about it, but this is a bit
	// less confusing to troubleshoot
	pendingMsgCount int32

	Closed bool
}

// conforms to RoundTripper, so we can use it for httpClient.Transport

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

func (m *MockHttpServer) Close() {
	m.Closed = true
	close(m.Request)
	close(m.Response)
}

func (m *MockHttpServer) GetPendingMessageCount() int {
	return int(m.pendingMsgCount)
}

func (m *MockHttpServer) ReceiveRequest() (*http.Request, bool) {
	atomic.AddInt32(&m.pendingMsgCount, 1)
	request := <-m.Request
	atomic.AddInt32(&m.pendingMsgCount, -1)
	return request, !m.Closed // reading from closed channels works fine and just returns the default
}

func (m *MockHttpServer) Respond(response *http.Response, err error) {
	if m.Closed {
		return // can't respond after closure
	}

	atomic.AddInt32(&m.pendingMsgCount, 1)
	m.Response <- responseOrError{response: response, error: err}
	atomic.AddInt32(&m.pendingMsgCount, -1)
}

// now we build some higher level methods on top of ReceiveRequest

func (m *MockHttpServer) ExpectRequest(t *testing.T, method string, pathAndQuery string) *RequestWrapper {
	r, ok := m.ReceiveRequest()
	if !ok { // this means the channel was closed
		// don't fatal, there'll be some other assertion failure too, and we want to let that have a chance to print
		t.Errorf("ExpectRequest %s %s failed; channel closed", method, pathAndQuery)
		return &RequestWrapper{&http.Request{}, m}
	}

	rPathAndQuery := r.URL.Path
	if r.URL.RawPath != "" { // RawPath may not be set, but if it is it should be used in preference to Path
		rPathAndQuery = r.URL.RawPath
	}
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

func (r *RequestWrapper) RespondWith(responseObject any) *RequestWrapper {
	r.RespondWithStatus(http.StatusOK, "200 OK", responseObject)
	return r
}

func (r *RequestWrapper) RespondWithJSON(jsonBytes []byte) *RequestWrapper {
	r.Server.Respond(&http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Body:          io.NopCloser(bytes.NewReader(jsonBytes)),
		ContentLength: int64(len(jsonBytes)),
		Header:        make(http.Header),
	}, nil)
	return r
}

func (r *RequestWrapper) ExpectHeader(t *testing.T, name string, value string) *RequestWrapper {
	assert.Contains(t, r.Request.Header, name)
	headerValues := r.Request.Header[name]
	assert.Contains(t, headerValues, value)
	return r
}

func (r *RequestWrapper) RespondWithStatus(statusCode int, statusString string, responseObject any) {
	var body []byte
	if responseObject != nil {
		b, err := json.Marshal(responseObject)
		if err != nil {
			panic(err) // you shouldn't feed unserializable stuff into RespondWithStatus
		}
		body = b
	} else {
		body = make([]byte, 0)
	}

	// Regarding response errors:
	// Note that we would use an error here for a low level thing like a network error.
	// An HTTP error like a 404 or 500 would be considered a valid response with an
	// appropriate status code
	r.Server.Respond(&http.Response{
		StatusCode:    statusCode,
		Status:        statusString,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}, nil)
}

func (r *RequestWrapper) RespondWithError(err error) {
	r.Server.Respond(nil, err)
}

func NewRootResource() *octopusApiClient.RootResource {
	root := octopusApiClient.NewRootResource()
	root.Links[constants.LinkSpaces] = "/api/spaces{/id}{?skip,ids,take,partialName}"

	// Note: all this stuff typically doesn't appear at the root resource level
	// has assigned a default space. We don't like default spaces, so the unit tests
	// should probably not mimic that structure; clean it up one day
	root.Links[constants.LinkChannels] = "/api/Spaces-1/channels{/id}{?skip,take,ids,partialName}"
	root.Links[constants.LinkDeploymentProcesses] = "/api/Spaces-1/deploymentprocesses{/id}{?skip,take,ids}"
	root.Links[constants.LinkEnvironments] = "/api/Spaces-1/environments{/id}{?name,skip,ids,take,partialName}"
	root.Links[constants.LinkFeeds] = "/api/Spaces-1/feeds{/id}{?skip,take,ids,partialName,feedType,name}"
	root.Links[constants.LinkProjects] = "/api/Spaces-1/projects{/id}{?name,skip,ids,clone,take,partialName,clonedFromProjectId}"
	root.Links[constants.LinkReleases] = "/api/Spaces-1/releases{/id}{?skip,ignoreChannelRules,take,ids}"
	root.Links[constants.LinkTenants] = "/api/Spaces-1/tenants{/id}{?skip,projectId,name,tags,take,ids,clone,partialName,clonedFromTenantId}"
	root.Links[constants.LinkAccounts] = "/api/Spaces-1/accounts{/id}{?skip,take,ids,partialName,accountType}"
	root.Links[constants.LinkPackages] = "/api/Spaces-1/packages{/id}{?nuGetPackageId,filter,latest,skip,take,includeNotes}"
	root.Links[constants.LinkLifecycles] = "/api/Spaces-1/lifecycles{/id}{?skip,take,ids,partialName}"
	root.Links[constants.LinkProjectGroups] = "/api/Spaces-1/projectgroups{/id}{?skip,take,ids,partialName}"
	root.Links[constants.LinkUsers] = "/api/users"
	root.Links[constants.LinkCurrentUser] = "/api/users/me"
	return root
}
