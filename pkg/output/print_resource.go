package output

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/OctopusDeploy/cli/pkg/constants"

	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/spf13/cobra"
)

func PrintResource[T any](item T, cmd *cobra.Command, mappers Mappers[T]) error {
	outputFormat := ResolveOutputFormat(cmd)

	switch outputFormat {
	case constants.OutputFormatJson:
		jsonMapper := mappers.Json
		if jsonMapper == nil {
			return errors.New("command does not support output in JSON format")
		}
		var outputJson any
		outputJson = jsonMapper(item)

		data, _ := json.MarshalIndent(outputJson, "", "  ")
		cmd.Println(string(data))

	case constants.OutputFormatBasic:
		textMapper := mappers.Basic
		if textMapper == nil {
			return errors.New("command does not support output in plain text")
		}
		cmd.Println(textMapper(item))

	case constants.OutputFormatTable, "": // table is the default of unspecified
		tableMapper := mappers.Table
		if tableMapper.Row == nil {
			return errors.New("command does not support output in table format")
		}

		t := NewTable(cmd.OutOrStdout())
		if tableMapper.Header != nil {
			for k, v := range tableMapper.Header {
				tableMapper.Header[k] = Bold(v)
			}
			t.AddRow(tableMapper.Header...)
		}

		t.AddRow(tableMapper.Row(item)...)

		return t.Print()

	default:
		return usage.NewUsageError(
			fmt.Sprintf("unsupported output format %s. Valid values are 'json', 'table', 'basic'. Defaults to table", outputFormat),
			cmd)
	}
	return nil
}
