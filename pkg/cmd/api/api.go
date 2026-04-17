package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/spf13/cobra"
)

func NewCmdAPI(f factory.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api <url>",
		Short: "Execute a raw API GET request",
		Long:  "Execute an authenticated GET request against the Octopus Server API and print the JSON response.",
		Example: heredoc.Docf(`
			%[1]s  api /api
			%[1]s  api /api/spaces
			%[1]s  api /api/Spaces-1/projects
		`, constants.ExecutableName),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return apiRun(cmd, f, args[0])
		},
		Annotations: map[string]string{
			annotations.IsCore: "true",
		},
	}

	return cmd
}

func apiRun(cmd *cobra.Command, f factory.Factory, path string) error {
	if err := validateAPIPath(path); err != nil {
		return err
	}

	client, err := f.GetSystemClient(apiclient.NewRequester(cmd))
	if err != nil {
		return err
	}

	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return err
	}

	resp, err := client.HttpSession().DoRawRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(string(body))
	}

	// Pretty-print if valid JSON, otherwise output raw
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
		cmd.Println(prettyJSON.String())
	} else {
		cmd.Print(string(body))
	}

	return nil
}

func validateAPIPath(path string) error {
	trimmed := strings.TrimLeft(path, "/")
	if !strings.HasPrefix(trimmed, "api") {
		return fmt.Errorf("the api command only supports paths prefixed with /api (e.g. /api/spaces)")
	}
	return nil
}
