package upload_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"github.com/MakeNowJust/heredoc/v2"
	cmdRoot "github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/test/fixtures"
	"github.com/OctopusDeploy/cli/test/testutil"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/packages"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/resources"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"strings"
	"testing"
)

var rootResource = testutil.NewRootResource()

func TestPackageUpload(t *testing.T) {
	const spaceID = "Spaces-1"
	space1 := fixtures.NewSpace(spaceID, "Default Space")

	// delta data is taken from octodiff tests so we can be sure it's correct
	createFileForDelta := func(modifyContents func([]byte)) []byte {
		contents, _ := base64.StdEncoding.DecodeString("MIICBDCCAaugAwIBAgIUGNg/B3GL5BId8KGNdhD6+NejvsQwCgYIKoZIzj0EAwIwWDELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxFzAVBgNVBAoMDk9jdG9wdXMgRGVwbG95MQwwCgYDVQQLDANSJkQxDTALBgNVBAMMBFRFU1QwHhcNMjMwMzIwMDk0ODQyWhcNMjQwMzE5MDk0ODQyWjBYMQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEXMBUGA1UECgwOT2N0b3B1cyBEZXBsb3kxDDAKBgNVBAsMA1ImRDENMAsGA1UEAwwEVEVTVDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFBLdySNg+Lj4gm7sil6Dk0k/0Xnnv+I3RZeZBmumFEtq9IhnaRuk9f/mNWhy4AxSlfzfQkx7PfzvUvOISz9LLqjUzBRMB0GA1UdDgQWBBS63SeKMeASd2r7/aTq2P3OkE8O/DAfBgNVHSMEGDAWgBS63SeKMeASd2r7/aTq2P3OkE8O/DAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0cAMEQCIFmc75IBFbZKfQvH3lWoS7p/Be54uekDr3y1K0pdzI6iAiAGV1RF2rnCEyWkjeO9fOUaNGEgFadGSHh8en4DJkU3cA==")
		if modifyContents != nil {
			modifyContents(contents)
		}
		return contents
	}

	// this is our "virtual filesystem". It's not really a VFS and we can't unit test path globbing at the moment, but it'll do
	files := map[string][]byte{
		"test.1.0.zip":  []byte("test1-contents"),
		"other.1.1.zip": []byte("other-contents"),
		// for delta upload tests
		"deltapkg.1.0.zip": createFileForDelta(nil),
		"deltapkg.2.0.zip": createFileForDelta(func(b []byte) {
			b[32] = 0xaa
			b[33] = 0xab
			b[34] = 0xac
		}),
	}
	opener := func(name string) (io.ReadSeekCloser, error) {
		if contents, ok := files[name]; ok {
			return &nopReadSeekCloser{inner: bytes.NewReader(contents)}, nil
		} else {
			return nil, os.ErrNotExist
		}
	}
	contextWithOpener := context.WithValue(context.TODO(), constants.ContextKeyOsOpen, opener)

	tests := []struct {
		name string
		run  func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer)
	}{
		{"requires at least one package", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload"})
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "at least one package must be specified")
			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"uploads a single package (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "test.1.0.zip", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)

			boundary := string(buf[2:62]) // the boundary will be random but is always in the same place/format so we can extract it
			assert.Equal(t, crlf(heredoc.Docf(`
			--%s
			Content-Disposition: form-data; name="file"; filename="test.1.0.zip"
			Content-Type: application/octet-stream
			
			test1-contents
			--%s--
			`, boundary, boundary)), string(buf[:bytesRead]))

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["test1.zip"]),
				Hash:             "some-hash",
				PackageId:        "test",
				Title:            "test.1.0",
				Version:          "1.0",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 200 means 'processed', we might ignored an existing file
			assert.Equal(t, "Uploaded package test.1.0.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"uploads multiple packages (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "-p", "test.1.0.zip", "--package", "other.1.1.zip", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			buf := make([]byte, 8192)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			bytesRead, err := req.Request.Body.Read(buf)
			require.Greater(t, bytesRead, 100)
			boundary := string(buf[2:62])
			assert.Equal(t, crlf(heredoc.Docf(`
			--%s
			Content-Disposition: form-data; name="file"; filename="test.1.0.zip"
			Content-Type: application/octet-stream
			
			test1-contents
			--%s--
			`, boundary, boundary)), string(buf[:bytesRead]))

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["test.1.0.zip"]),
				Hash:             "some-hash",
				PackageId:        "test",
				Title:            "test.1.0",
				Version:          "1.0",
				Resource:         *resources.NewResource(),
			})

			// ----

			req = api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			bytesRead, err = req.Request.Body.Read(buf)
			require.Greater(t, bytesRead, 100)
			boundary = string(buf[2:62])
			assert.Equal(t, crlf(heredoc.Docf(`
			--%s
			Content-Disposition: form-data; name="file"; filename="other.1.1.zip"
			Content-Type: application/octet-stream
			
			other-contents
			--%s--
			`, boundary, boundary)), string(buf[:bytesRead]))

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["other.1.1.zip"]),
				Hash:             "some-hash",
				PackageId:        "other",
				Title:            "other.1.1",
				Version:          "1.1",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 201 means 'created', we wrote the file
			assert.Equal(t, "Uploaded package test.1.0.zip\nUploaded package other.1.1.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"sets overwriteMode (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "test.1.0.zip", "--overwrite-mode", "overwrite", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=OverwriteExisting")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)
			assert.Equal(t, 258, bytesRead)

			req.RespondWithStatus(200, "200 OK", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["test1.zip"]),
				Hash:             "some-hash",
				PackageId:        "test",
				Title:            "test.1.0",
				Version:          "1.0",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 200 means 'processed', we might ignored an existing file
			assert.Equal(t, "Ignored existing package test.1.0.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"uploads multiple packages; default behaviour of failing on first error (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "-p", "test.1.0.zip", "--package", "other.1.1.zip", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			req.RespondWithStatus(400, "400 Bad Request", struct{ ErrorMessage string }{ErrorMessage: "the package is not gluten-free"})

			// the CLI exits here, no further requests are made

			_, err := testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "Octopus API error: the package is not gluten-free [] ")
			// http status of 201 means 'created', we wrote the file
			assert.Equal(t, "", stdOut.String())
			assert.Equal(t, "Failed to upload package test.1.0.zip - Octopus API error: the package is not gluten-free [] \n", stdErr.String())
		}},

		{"uploads multiple packages; --continue-on-error (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "-p", "test.1.0.zip", "--package", "other.1.1.zip", "--continue-on-error", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			// fail on the first file
			req.RespondWithStatus(400, "400 Bad Request", struct{ ErrorMessage string }{ErrorMessage: "the package is not gluten-free"})

			// continue on to the next file

			req = api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)
			assert.Equal(t, 259, bytesRead)

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["other.1.1.zip"]),
				Hash:             "some-hash",
				PackageId:        "other",
				Title:            "other.1.1",
				Version:          "1.1",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "one or more packages failed to upload")
			// http status of 201 means 'created', we wrote the file
			assert.Equal(t, "Uploaded package other.1.1.zip\n", stdOut.String())
			assert.Equal(t, "Failed to upload package test.1.0.zip - Octopus API error: the package is not gluten-free [] \n", stdErr.String())
		}},

		{"uploads multiple packages; doesn't upload the same file more than once (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "-p", "test.1.0.zip", "--package", "test.1.0.zip", "test.1.0.zip", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)
			assert.Equal(t, 258, bytesRead)

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["test.1.0.zip"]),
				Hash:             "some-hash",
				PackageId:        "test",
				Title:            "test.1.0",
				Version:          "1.0",
				Resource:         *resources.NewResource(),
			})

			// no further uploads

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 201 means 'created', we wrote the file
			assert.Equal(t, "Uploaded package test.1.0.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"output-format=json, uploads multiple packages; --continue-on-error (delta disabled)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "-p", "test.1.0.zip", "--package", "other.1.1.zip", "--continue-on-error", "--output-format", "json", "--use-delta-compression", "false"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			// fail on the first file
			req.RespondWithStatus(400, "400 Bad Request", struct{ ErrorMessage string }{ErrorMessage: "the package is not gluten-free"})

			// continue on to the next file

			req = api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)
			assert.Equal(t, 259, bytesRead)

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["other.1.1.zip"]),
				Hash:             "some-hash",
				PackageId:        "other",
				Title:            "other.1.1",
				Version:          "1.1",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.EqualError(t, err, "one or more packages failed to upload")
			type sr struct {
				PackagePath string `json:"package,omitempty"`
			}
			type fr struct {
				PackagePath string `json:"package,omitempty"`
				Error       string `json:"error,omitempty"`
			}
			type xr struct {
				Succeeded []sr `json:"succeeded,omitempty"`
				Failed    []fr `json:"failed,omitempty"`
			}
			parsedStdout, err := testutil.ParseJsonStrict[xr](stdOut)
			assert.Nil(t, err)

			assert.Equal(t, xr{
				Succeeded: []sr{{PackagePath: "other.1.1.zip"}},
				Failed:    []fr{{PackagePath: "test.1.0.zip", Error: "Octopus API error: the package is not gluten-free [] "}},
			}, parsedStdout)
			assert.Equal(t, "", stdErr.String())
		}},

		{"uploads a single package (delta enabled, no baseline so fallback to full upload)", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "test.1.0.zip"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			req := api.ExpectRequest(t, "GET", "/api/Spaces-1/packages/test/1.0/delta-signature")
			req.RespondWithStatus(404, "404 Not Found", nil)

			// now it does a regular upload
			req = api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/raw?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)

			boundary := string(buf[2:62]) // the boundary will be random but is always in the same place/format so we can extract it
			assert.Equal(t, crlf(heredoc.Docf(`
			--%s
			Content-Disposition: form-data; name="file"; filename="test.1.0.zip"
			Content-Type: application/octet-stream
			
			test1-contents
			--%s--
			`, boundary, boundary)), string(buf[:bytesRead]))

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["test1.zip"]),
				Hash:             "some-hash",
				PackageId:        "test",
				Title:            "test.1.0",
				Version:          "1.0",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 200 means 'processed', we might ignored an existing file
			assert.Equal(t, "Uploaded package test.1.0.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},

		{"uploads a package using delta compression", func(t *testing.T, api *testutil.MockHttpServer, rootCmd *cobra.Command, stdOut *bytes.Buffer, stdErr *bytes.Buffer) {
			// in this scenario the server pretends it already has deltapkg.1.0 and returns the signature for it
			// and we are uploading deltapkg.2.0 (which is in our virtual filesystem, see top)
			cmdReceiver := testutil.GoBegin2(func() (*cobra.Command, error) {
				defer api.Close()
				rootCmd.SetArgs([]string{"package", "upload", "deltapkg.2.0.zip"})
				rootCmd.SetContext(contextWithOpener)
				return rootCmd.ExecuteC()
			})

			api.ExpectRequest(t, "GET", "/api").RespondWith(rootResource)
			api.ExpectRequest(t, "GET", "/api/Spaces-1").RespondWith(rootResource)

			// in this instance we pretend that file 1.0 was our fake testdata from octodiff.test
			// which is base64 MIICBDCCAaugAwIBAgIUGNg/B3GL5BId8KGNdhD6+NejvsQwCgYIKoZIzj0EAwIwWDELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxFzAVBgNVBAoMDk9jdG9wdXMgRGVwbG95MQwwCgYDVQQLDANSJkQxDTALBgNVBAMMBFRFU1QwHhcNMjMwMzIwMDk0ODQyWhcNMjQwMzE5MDk0ODQyWjBYMQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEXMBUGA1UECgwOT2N0b3B1cyBEZXBsb3kxDDAKBgNVBAsMA1ImRDENMAsGA1UEAwwEVEVTVDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFBLdySNg+Lj4gm7sil6Dk0k/0Xnnv+I3RZeZBmumFEtq9IhnaRuk9f/mNWhy4AxSlfzfQkx7PfzvUvOISz9LLqjUzBRMB0GA1UdDgQWBBS63SeKMeASd2r7/aTq2P3OkE8O/DAfBgNVHSMEGDAWgBS63SeKMeASd2r7/aTq2P3OkE8O/DAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0cAMEQCIFmc75IBFbZKfQvH3lWoS7p/Be54uekDr3y1K0pdzI6iAiAGV1RF2rnCEyWkjeO9fOUaNGEgFadGSHh8en4DJkU3cA==

			// we request a signature file, which is (per octodiff tests)
			signatureBase64 := "T0NUT1NJRwEEU0hBMQdBZGxlcjMyPj4+CAL3n6LwMwvQaYLTtdvabBpq0WaHoM2wPA0="
			signatureResponse := map[string]any{"baseVersion": "1.0", "signature": signatureBase64}

			api.ExpectRequest(t, "GET", "/api/Spaces-1/packages/deltapkg/2.0/delta-signature").RespondWith(signatureResponse)

			// now it should go and open deltapkg.2.0.zip from the virtual filesystem, calculate the delta, and post it back

			req := api.ExpectRequest(t, "POST", "/api/Spaces-1/packages/deltapkg/1.0/delta?overwriteMode=FailIfExists")

			buf := make([]byte, 8192)
			bytesRead, err := req.Request.Body.Read(buf)

			boundary := string(buf[2:62]) // the boundary will be random but is always in the same place/format so we can extract it

			expectedHeader := crlf(heredoc.Docf(`
			--%s
			Content-Disposition: form-data; name="file"; filename="deltapkg.2.0.zip"
			Content-Type: application/octet-stream
			
			`, boundary))

			expectedDelta, _ := base64.StdEncoding.DecodeString("T0NUT0RFTFRBAQRTSEExFAAAABS6ZO6rrSld1gz6vWSP8Xa2SJB3Pj4+gAgCAAAAAAAAMIICBDCCAaugAwIBAgIUGNg/B3GL5BId8KGNdhD6+Neqq6wwCgYIKoZIzj0EAwIwWDELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxFzAVBgNVBAoMDk9jdG9wdXMgRGVwbG95MQwwCgYDVQQLDANSJkQxDTALBgNVBAMMBFRFU1QwHhcNMjMwMzIwMDk0ODQyWhcNMjQwMzE5MDk0ODQyWjBYMQswCQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEXMBUGA1UECgwOT2N0b3B1cyBEZXBsb3kxDDAKBgNVBAsMA1ImRDENMAsGA1UEAwwEVEVTVDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFBLdySNg+Lj4gm7sil6Dk0k/0Xnnv+I3RZeZBmumFEtq9IhnaRuk9f/mNWhy4AxSlfzfQkx7PfzvUvOISz9LLqjUzBRMB0GA1UdDgQWBBS63SeKMeASd2r7/aTq2P3OkE8O/DAfBgNVHSMEGDAWgBS63SeKMeASd2r7/aTq2P3OkE8O/DAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0cAMEQCIFmc75IBFbZKfQvH3lWoS7p/Be54uekDr3y1K0pdzI6iAiAGV1RF2rnCEyWkjeO9fOUaNGEgFadGSHh8en4DJkU3cA==")

			expectedTrailer := crlf(heredoc.Docf(`

			--%s--
			`, boundary))

			expectedFullBody := append(
				append([]byte(expectedHeader), expectedDelta...),
				[]byte(expectedTrailer)...)

			assert.Equal(t, hex.EncodeToString(expectedFullBody), hex.EncodeToString(buf[:bytesRead]))

			req.RespondWithStatus(201, "201 Created", &packages.PackageUploadResponse{
				PackageSizeBytes: len(files["deltapkg.2.0.zip"]),
				Hash:             "some-hash",
				PackageId:        "deltapkg",
				Title:            "deltapkg.2.0",
				Version:          "2.0",
				Resource:         *resources.NewResource(),
			})

			_, err = testutil.ReceivePair(cmdReceiver)
			assert.Nil(t, err)
			// http status of 200 means 'processed', we might ignored an existing file
			assert.Equal(t, "Uploaded package deltapkg.2.0.zip\n", stdOut.String())
			assert.Equal(t, "", stdErr.String())
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
			api := testutil.NewMockHttpServer()
			fac := testutil.NewMockFactoryWithSpaceAndPrompt(api, space1, nil)
			rootCmd := cmdRoot.NewCmdRoot(fac, nil, nil)
			rootCmd.SetOut(stdout)
			rootCmd.SetErr(stderr)
			test.run(t, api, rootCmd, stdout, stderr)
		})
	}
}

// TODO an integration test which tests globbing by creating files in the tmp dir and sending them to a real server

func crlf(text string) string {
	return strings.ReplaceAll(text, "\n", "\r\n")
}

// no-ops the close method on an existing ReadSeeker for cases where it doesn't matter (e.g. in memory byte array)
type nopReadSeekCloser struct {
	inner io.ReadSeeker
}

func (c *nopReadSeekCloser) Read(p []byte) (n int, err error) {
	return c.inner.Read(p)
}

func (c *nopReadSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return c.inner.Seek(offset, whence)
}

func (c *nopReadSeekCloser) Close() error {
	return nil // deliberate do-nothing
}

var _ io.ReadSeekCloser = (*nopReadSeekCloser)(nil)
