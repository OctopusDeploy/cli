package list

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/util/flag"
	"github.com/OctopusDeploy/go-octopusdeploy/v2/pkg/core"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

type ApiFlags struct {
	Method *flag.Flag[string]
	Header *flag.Flag[[]string]
	Data   *flag.Flag[string]
}

func NewApiFlags() *ApiFlags {
	return &ApiFlags{
		Method: flag.New[string]("method", false),
		Header: flag.New[[]string]("header", false),
		Data:   flag.New[string]("data", false),
	}
}

func NewCmdApi(f factory.Factory) *cobra.Command {
	apiFlags := NewApiFlags()

	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Send API request",
		Long:  "Send an API request to Octopus Server",
		Example: heredoc.Docf(`
			# Get the current logged in user
			$ %[1]s api users/me

			# Create a new environment
			$ %[1]s api -X POST -d '{\"name\":\"Demo environment from CLI\"}' spaces/Spaces-1/environments
		`, constants.ExecutableName),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("endpoint must be supplied")
			}

			url, err := url.Parse(args[0])
			if url.IsAbs() {
				return errors.New("endpoint cannot be absolute")
			}

			if err != nil {
				return err
			}

			client, err := f.GetSystemClient(apiclient.NewRequester(cmd))

			if err != nil {
				return err
			}

			httpSession := client.HttpSession()
			fullUrl := httpSession.BaseURL.JoinPath(url.Path)
			fullUrl.RawQuery = url.RawQuery

			req := &http.Request{
				Method: apiFlags.Method.Value,
				URL:    fullUrl,
			}

			if apiFlags.Header.Value != nil {
				for _, headerValue := range apiFlags.Header.Value {
					headerPair := strings.Split(headerValue, ":")

					if len(headerPair) != 2 {
						return errors.New("header must be in format key:value")
					}

					req.Header.Add(headerPair[0], headerPair[1])
				}
			}

			var bodyPayload = new(map[string]any)
			var responsePayload = new(map[string]any)
			var errorPayload = new(core.APIError)

			if apiFlags.Data.Value != "" {
				err = json.Unmarshal([]byte(apiFlags.Data.Value), bodyPayload)

				if err != nil {
					return err
				}
			}

			_, err = httpSession.DoRawJsonRequest(req, bodyPayload, responsePayload, errorPayload)

			if err != nil {
				return err
			}

			data, _ := json.MarshalIndent(responsePayload, "", "  ")
			fmt.Println(string(data))

			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&apiFlags.Method.Value, apiFlags.Method.Name, "X", "GET", "The HTTP method for the request (default \"GET\")")
	flags.StringArrayVarP(&apiFlags.Header.Value, apiFlags.Header.Name, "H", nil, "Add a HTTP request header in key:value format")
	flags.StringVarP(&apiFlags.Data.Value, apiFlags.Data.Name, "d", "", "The body for the request, formatted as JSON")

	return cmd
}
