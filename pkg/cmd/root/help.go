package root

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	version "github.com/OctopusDeploy/cli"
	"github.com/OctopusDeploy/cli/pkg/constants"
	"github.com/OctopusDeploy/cli/pkg/constants/annotations"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func isRootCmd(cmd *cobra.Command) bool {
	return cmd != nil && !cmd.HasParent()
}

func rootHelpFunc(cmd *cobra.Command, _ []string) {
	namePadding := calculatePadding(cmd)
	coreCmds := []string{}
	configurationCmds := []string{}
	libraryCmds := []string{}
	infrastructureCmds := []string{}
	additionalCmds := []string{}

	flags := cmd.Flags()

	if isRootCmd(cmd) {
		out := cmd.OutOrStdout()
		if versionVal, err := flags.GetBool(constants.FlagHelp); err == nil && versionVal {
			fmt.Fprintln(out, strings.TrimSpace(version.Version))
			return
		} else if err != nil {
			fmt.Fprintln(out, err)
			return
		}
	}

	for _, c := range cmd.Commands() {
		if c.Short == "" {
			continue
		}
		if c.Hidden {
			continue
		}
		s := rpad(c.Name()+":", namePadding) + c.Short
		if _, ok := c.Annotations[annotations.IsCore]; ok {
			coreCmds = append(coreCmds, s)
		} else if _, ok := c.Annotations[annotations.IsConfiguration]; ok {
			configurationCmds = append(configurationCmds, s)
		} else if _, ok := c.Annotations[annotations.IsLibrary]; ok {
			libraryCmds = append(libraryCmds, s)
		} else if _, ok := c.Annotations[annotations.IsInfrastructure]; ok {
			infrastructureCmds = append(infrastructureCmds, s)
		} else {
			additionalCmds = append(additionalCmds, s)
		}
	}

	// Assume all additional cmds are core if no core cmds are found
	if len(coreCmds) == 0 {
		coreCmds = additionalCmds
		additionalCmds = []string{}
	}

	type helpEntry struct {
		Title string
		Body  string
	}

	longText := cmd.Long
	if longText == "" {
		longText = cmd.Short
	}

	helpEntries := []helpEntry{}
	if longText != "" {
		helpEntries = append(helpEntries, helpEntry{"", longText})
	}
	if isRootCmd(cmd) && viper.GetBool(constants.ConfigShowOctopus) {
		helpEntries = append(helpEntries, helpEntry{"", output.Bluef("%s", constants.OctopusLogo)})
	}
	helpEntries = append(helpEntries, helpEntry{"USAGE", cmd.UseLine()})
	if len(coreCmds) > 0 {
		helpEntries = append(helpEntries, helpEntry{"CORE COMMANDS", strings.Join(coreCmds, "\n")})
	}
	if len(configurationCmds) > 0 {
		helpEntries = append(helpEntries, helpEntry{"CONFIGURATION COMMANDS", strings.Join(configurationCmds, "\n")})
	}
	if len(libraryCmds) > 0 {
		helpEntries = append(helpEntries, helpEntry{"LIBRARY COMMANDS", strings.Join(libraryCmds, "\n")})
	}
	if len(infrastructureCmds) > 0 {
		helpEntries = append(helpEntries, helpEntry{"INFRASTRUCTURE COMMANDS", strings.Join(infrastructureCmds, "\n")})
	}
	if len(additionalCmds) > 0 {
		helpEntries = append(helpEntries, helpEntry{"ADDITIONAL COMMANDS", strings.Join(additionalCmds, "\n")})
	}

	flagsUsage := cmd.LocalFlags().FlagUsages()
	if flagsUsage != "" {
		helpEntries = append(helpEntries, helpEntry{"FLAGS", dedent(flagsUsage)})
	}
	inheritedFlagUsage := cmd.InheritedFlags().FlagUsages()
	if inheritedFlagUsage != "" {
		helpEntries = append(helpEntries, helpEntry{"INHERITED FLAGS", dedent(inheritedFlagUsage)})
	}

	if !isRootCmd(cmd) {
		if cmd.HasExample() {
			helpEntries = append(helpEntries, helpEntry{"EXAMPLE", cmd.Example})
		}
	}

	if isRootCmd(cmd) {
		helpEntries = append(helpEntries, helpEntry{"LEARN MORE", `
  Use "octopus [command] [subcommand] --help" for information about a command.`})
		helpEntries = append(helpEntries, helpEntry{"FEEDBACK", "Open an issue: https://github.com/OctopusDeploy/cli/issues"})
	}

	out := cmd.OutOrStdout()
	for _, e := range helpEntries {
		if e.Title != "" {
			fmt.Fprintln(out, output.Bold(e.Title))
			fmt.Fprintln(out, Indent(strings.Trim(e.Body, "\r\n"), " "))
		} else {
			fmt.Fprintln(out, e.Body)
		}
		fmt.Fprintln(out)
	}
}

func calculatePadding(cmd *cobra.Command) int {
	namePadding := 12
	for _, c := range cmd.Commands() {
		if len(c.Name()) > namePadding {
			namePadding = len(c.Name()) + 2
		}
	}

	return namePadding
}

func rpad(s string, padding int) string {
	template := fmt.Sprintf("%%-%ds ", padding)
	return fmt.Sprintf(template, s)
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	minIndent := -1

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		indent := len(l) - len(strings.TrimLeft(l, " "))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}

	if minIndent <= 0 {
		return s
	}

	var buf bytes.Buffer
	for _, l := range lines {
		fmt.Fprintln(&buf, strings.TrimPrefix(l, strings.Repeat(" ", minIndent)))
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

var lineRE = regexp.MustCompile(`(?m)^`)

func Indent(s, indent string) string {
	if len(strings.TrimSpace(s)) == 0 {
		return s
	}
	return lineRE.ReplaceAllLiteralString(s, indent)
}
