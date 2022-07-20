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

func PrintArray[T any](items []T, cmd *cobra.Command, jsonMapper func(item T) any, stringMapper func(item T) string) error {
	outputFormat, _ := cmd.Flags().GetString("outputFormat")
	switch strings.ToLower(outputFormat) {

	case "json":
		var outputJson []any
		for _, e := range items {
			outputJson = append(outputJson, jsonMapper(e))
		}

		data, _ := json.MarshalIndent(outputJson, "", "  ")
		fmt.Println(string(data))
	case "":
		itemCount := len(items)
		if itemCount == 1 {
			fmt.Printf("1 item found.\n")
		} else {
			fmt.Printf("%d items found.\n", itemCount)
		}
		for _, e := range items {
			fmt.Println(stringMapper(e)) // TODO this would become fancy and feed into a table
		}
	default:
		return errors.New(fmt.Sprintf("Unsupported outputFormat %s. Valid values are 'json' or an empty string", outputFormat))
	}
	return nil
}
