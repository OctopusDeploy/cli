package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
	outputFormat, _ := cmd.Flags().GetString("outputFormat")

	switch strings.ToLower(outputFormat) {
	case "json":
		jsonMapper := mappers.Json
		if jsonMapper == nil {
			return errors.New("Command does not support output in JSON format")
		}
		var outputJson []any
		for _, e := range items {
			outputJson = append(outputJson, jsonMapper(e))
		}

		data, _ := json.MarshalIndent(outputJson, "", "  ")
		fmt.Println(string(data))

	case "basic", "text":
		textMapper := mappers.Basic
		if textMapper == nil {
			return errors.New("Command does not support output in plain text")
		}
		for _, e := range items {
			fmt.Println(textMapper(e))
		}

	case "table", "": // table is the default of unspecified
		tableMapper := mappers.Table
		if tableMapper.Row == nil {
			return errors.New("Command does not support output in table format")
		}

		ioWriter := cmd.OutOrStdout()
		t := NewTable(ioWriter)
		if tableMapper.Header != nil {
			t.AddRow(tableMapper.Header...)

			headerSeparators := []string{}
			for _, h := range tableMapper.Header {
				headerSeparators = append(headerSeparators, strings.Repeat("-", len(h)))
			}
			t.AddRow(headerSeparators...)
		}

		for _, item := range items {
			t.AddRow(tableMapper.Row(item)...)
		}

		return t.Print()

	default:
		return errors.New(fmt.Sprintf("Unsupported outputFormat %s. Valid values are 'json', 'table', 'basic'. Defaults to table", outputFormat))
	}
	return nil
}
