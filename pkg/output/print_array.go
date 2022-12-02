package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/OctopusDeploy/cli/pkg/constants"

	"github.com/OctopusDeploy/cli/pkg/usage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Common struct used for rendering JSON summaries of things that just have an ID and a Name
type IdAndName struct {
	Id   string `json:"Id"`
	Name string `json:"Name"`
}

type TableDefinition[T any] struct {
	Header []string
	Row    func(item T) []string
}

// carries conversion functions used by PrintArray and potentially other output code in future
type Mappers[T any] struct {
	// A function which will convert T into an output structure suitable for json.Marshal (e.g. IdAndName).
	// If you leave this as nil, then the command will simply not support output as JSON and will
	// fail if someone asks for it
	Json func(item T) any

	// A function which will convert T into ?? suitable for table printing
	// If you leave this as nil, then the command will simply not support output as
	// a table and will fail if someone asks for it
	Table TableDefinition[T]

	// A function which will convert T into a string suitable for basic text display
	// If you leave this as nil, then the command will simply not support output as basic text and will
	// fail if someone asks for it
	Basic func(item T) string

	// NOTE: We might have some kinds of entities where table formatting doesn't make sense, and we want to
	// render those as basic text instead. This seems unlikely though, defer it until the issue comes up.

	// NOTE: The structure for printing tables would also work for CSV... perhaps we can have --outputFormat=csv for free?
}

func PrintArray[T any](items []T, cmd *cobra.Command, mappers Mappers[T]) error {
	outputFormat, _ := cmd.Flags().GetString(constants.FlagOutputFormat)
	if outputFormat == "" {
		outputFormat = viper.GetString(constants.ConfigOutputFormat)
	}

	switch strings.ToLower(outputFormat) {
	case constants.OutputFormatJson:
		jsonMapper := mappers.Json
		if jsonMapper == nil {
			return errors.New("command does not support output in JSON format")
		}
		var outputJson []any
		for _, e := range items {
			outputJson = append(outputJson, jsonMapper(e))
		}

		data, _ := json.MarshalIndent(outputJson, "", "  ")
		cmd.Println(string(data))

	case constants.OutputFormatBasic:
		textMapper := mappers.Basic
		if textMapper == nil {
			return errors.New("command does not support output in plain text")
		}
		for _, e := range items {
			cmd.Println(textMapper(e))
		}

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

		for _, item := range items {
			t.AddRow(tableMapper.Row(item)...)
		}

		return t.Print()

	default:
		return usage.NewUsageError(
			fmt.Sprintf("unsupported output format %s. Valid values are 'json', 'table', 'basic'. Defaults to table", outputFormat),
			cmd)
	}
	return nil
}
