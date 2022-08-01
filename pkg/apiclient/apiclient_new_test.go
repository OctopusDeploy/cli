package apiclient_test

import (
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/spaces"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient_GetSpacedClient_NoPrompt_NewStyle(t *testing.T) {
	root := testutil.NewRootResource()
	integrationsSpace := spaces.NewSpace("Integrations")
	integrationsSpace.ID = "Spaces-7"

	cloudSpace := spaces.NewSpace("Cloud")
	cloudSpace.ID = "Spaces-39"

	spaceNotSpecifiedMessage := "space must be specified when not running interactively; please set the OCTOPUS_SPACE environment variable or specify --space on the command line"

	t.Run("GetSpacedClient returns an error when no space is specified and only one space exists", func(t *testing.T) {
		// this would pass in interactive mode; we'd auto select the space, however we don't want to do
		// that in no-prompt mode because otherwise people could write a CI script that worked due to
		// auto-selection of the first space, which would then unexpectedly break later if someone added a
		// second space to the octopus server

		api := testutil.NewMockHttpServer()

		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "", nil)

		p := testutil.GoBeginFunc2(factory2.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		apiClient, err := p.Wait()

		assert.Nil(t, apiClient)
		assert.Equal(t, spaceNotSpecifiedMessage, err.Error()) // some strongly-typed errors would probably be nicer
		// assert.Equal(t, 0, rt.RemainingQueueLength())
		// TODO assert that all the channels are shutdown
	})

	t.Run("GetSpacedClient will select by name in preference to ID where there is a collision", func(t *testing.T) {
		missedSpace := spaces.NewSpace("Missed")
		missedSpace.ID = "Spaces-7"

		spaces7space := spaces.NewSpace("Spaces-7") // nobody would do this in reality, but our software must still work properly
		spaces7space.ID = "Spaces-209"

		api := testutil.NewMockHttpServer()
		factory2, _ := apiclient.NewClientFactory(testutil.NewMockHttpClientWithTransport(api), "http://server", placeholderApiKey, "Spaces-7", nil)

		p := testutil.GoBeginFunc2(factory2.GetSpacedClient)

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/spaces/all").RespondWith([]*spaces.Space{
			missedSpace,
			spaces7space,
		})

		api.ExpectRequest(t, "GET", "/api").RespondWith(root)

		api.ExpectRequest(t, "GET", "/api/Spaces-209").RespondWith(spaces7space)

		apiClient, err := p.Wait()

		assert.Nil(t, err)
		assert.NotNil(t, apiClient)
		assert.Equal(t, 0, api.GetPendingMessageCount())
	})
}

// -------------------
//
//type FakeApiResponder struct {
//	responderQueue []FakeRequestHandler
//}
//
//// conforms to RoundTripper so we can use it for httpClient.Transport
//
//func (f *FakeApiResponder) RoundTrip(r *http.Request) (*http.Response, error) {
//	handler := f.tryDequeueHandler()
//	if handler == nil {
//		return nil, errors.New(fmt.Sprintf("Test Failure! No responder for %s %s", r.Method, r.URL))
//	}
//
//	pathAndQuery := r.URL.Path
//	if r.URL.RawQuery != "" {
//		pathAndQuery = fmt.Sprintf("%s?%s", pathAndQuery, r.URL.RawQuery)
//	}
//	if handler.Method != r.Method || handler.PathAndQuery != pathAndQuery {
//		return nil, errors.New(fmt.Sprintf("Test Failure! Client sent %s %s, have responder for %s %s", r.Method, pathAndQuery, handler.Method, handler.PathAndQuery))
//	}
//
//	// if we get here we are good to go
//	return handler.Handler(r)
//}
//
//// higher level helper where your lambda can just return 'any' and it will get serialised into JSON and
//// sent as a response
//func (f *FakeApiResponder) EnqueueResponder(method string, pathAndQuery string, handler func(r *http.Request) (any, error)) {
//	rawHandler := func(r *http.Request) (*http.Response, error) {
//		responseObject, err := handler(r)
//		if err != nil {
//			return nil, err
//		}
//
//		body, err := json.Marshal(responseObject)
//		if err != nil {
//			return nil, err
//		}
//
//		return &http.Response{
//			StatusCode:    http.StatusOK,
//			Body:          ioutil.NopCloser(bytes.NewReader(body)),
//			ContentLength: int64(len(body)),
//		}, nil
//	}
//
//	fh := FakeRequestHandler{
//		Method:       method,
//		PathAndQuery: pathAndQuery,
//		Handler:      rawHandler,
//	}
//	f.responderQueue = append(f.responderQueue, fh)
//}
//
//func (f *FakeApiResponder) EnqueueRawResponder(method string, pathAndQuery string, handler func(r *http.Request) (*http.Response, error)) {
//	fh := FakeRequestHandler{
//		Method:       method,
//		PathAndQuery: pathAndQuery,
//		Handler:      handler,
//	}
//	f.responderQueue = append(f.responderQueue, fh)
//}
//
//// returns a pointer to the dequeued handler. Nil if the queue is empty
//func (f *FakeApiResponder) tryDequeueHandler() *FakeRequestHandler {
//	if f.responderQueue == nil || len(f.responderQueue) == 0 {
//		return nil
//	}
//
//	head, rest := f.responderQueue[0], f.responderQueue[1:]
//	f.responderQueue = rest
//	return &head
//}
//
//// RemainingQueueLength returns the number of unprocessed responders. You should always
//// assert that this is zero at the end of a test to ensure that you haven't succeeded accidentally.
//func (f *FakeApiResponder) RemainingQueueLength() int {
//	return len(f.responderQueue)
//}
//
//func NewFakeApiResponder() *FakeApiResponder {
//	return &FakeApiResponder{responderQueue: []FakeRequestHandler{}}
//}
