package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/AlecAivazis/survey/v2"
	version "github.com/OctopusDeploy/cli"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

type TemplateInformation struct {
	Title    string
	Command  *cobra.Command
	Filename string
	Position int
}

type Pages []*TemplateInformation

type PageCollection struct {
	Pages *Pages
}

func (p *Pages) Len() int {
	return len(*p)
}

func (p *Pages) Less(i, j int) bool {
	return (*p)[i].Position < (*p)[j].Position
}

func (p *Pages) Swap(i, j int) {
	(*p)[i], (*p)[j] = (*p)[j], (*p)[i]
}

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := pflag.NewFlagSet("", pflag.ContinueOnError)
	manPage := flags.BoolP("man-page", "", false, "Generate manual pages")
	website := flags.BoolP("website", "", false, "Generate website pages")
	dir := flags.StringP("doc-path", "", "", "Path directory where you want generate doc files")
	help := flags.BoolP("help", "h", false, "Help about any command")

	if err := flags.Parse(args); err != nil {
		return err
	}

	if *help {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n\n%s", filepath.Base(args[0]), flags.FlagUsages())
		return nil
	}

	if *dir == "" {
		return fmt.Errorf("error: --doc-path not set")
	}

	askProvider := question.NewAskProvider(survey.AskOne)
	clientFactory := apiclient.NewStubClientFactory()
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithColor("cyan"))
	buildVersion := strings.TrimSpace(version.Version)
	f := factory.New(clientFactory, askProvider, s, buildVersion)

	cmd := root.NewCmdRoot(f, clientFactory, askProvider)
	cmd.DisableAutoGenTag = true
	cmd.InitDefaultHelpCmd()

	if strings.HasPrefix(*dir, "~/") {
		usr, _ := user.Current()
		home := usr.HomeDir
		*dir = filepath.Join(home, (*dir)[2:])
	}

	if _, err := os.Stat(*dir); !os.IsNotExist(err) {
		err := RemoveContents(*dir)
		if err != nil {
			return err
		}
	}

	if err := os.MkdirAll(*dir, 0755); err != nil {
		return err
	}

	if *website {
		position := 0
		pageCollection := &PageCollection{Pages: &Pages{}}
		if err := GenMarkdownTreeCustom(cmd, *dir, &position, pageCollection); err != nil {
			return err
		}

		filename := filepath.Join(*dir, "index.md")
		f, err := os.Create(filename)

		if err != nil {
			return err
		}
		defer f.Close()
		t := template.Must(template.New("index-template").Parse(indexTemplate))
		sort.Sort(pageCollection.Pages)
		t.Execute(f, pageCollection)
		return nil
	}

	header := &doc.GenManHeader{
		Title:   "MINE",
		Section: "3",
	}

	if *manPage {
		if err := doc.GenManTree(cmd, header, *dir); err != nil {
			return err
		}
	}

	return nil
}

func GenMarkdownCustom(cmd *cobra.Command, w io.Writer, info TemplateInformation) error {
	cmd.InitDefaultHelpCmd()
	cmd.InitDefaultHelpFlag()

	t := template.Must(template.New("documentation-template").Parse(documentationTemplate))
	return t.Execute(w, info)
}

func GenMarkdownTreeCustom(cmd *cobra.Command, dir string, positionCounter *int, pageCollection *PageCollection) error {
	myPosition := *positionCounter
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}

		*positionCounter++

		if err := GenMarkdownTreeCustom(c, dir, positionCounter, pageCollection); err != nil {
			return err
		}

	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "-") + ".md"
	filename := filepath.Join(dir, basename)

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	info := TemplateInformation{
		Title:    cmd.CommandPath(),
		Command:  cmd,
		Position: myPosition,
		Filename: basename,
	}

	*pageCollection.Pages = append(*pageCollection.Pages, &info)

	if err := GenMarkdownCustom(cmd, f, info); err != nil {
		return err
	}
	return nil
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

const documentationTemplate = `---
title: {{.Title}}
description: {{.Command.Short}}
position: {{.Position}}
---

{{.Command.Long}}

` + "\n```text" + `{{define "T1"}}Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{.Name }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{.Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{.Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{.CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}{{end}}
{{template "T1" .Command}}` + "\n```\n" + `
{{- if .Command.Example }}

## Examples

!include <samples-instance>

` + "\n```text" + `
{{ .Command.Example }}
` + "\n```\n" + `
{{- end }}

## Learn more

- [Octopus CLI](/docs/octopus-rest-api/cli/index.md)
- [Creating API keys](/docs/octopus-rest-api/how-to-create-an-api-key.md)`

const indexTemplate = `---
title: CLI
description: The all-new Octopus CLI
position: 100
hideInThisSection: true
---

:::hint
The new Octopus CLI is currently an EAP release
:::

The Octopus CLI is a command line tool that builds on top of the [Octopus Deploy REST API](/docs/octopus-rest-api/index.md). With the Octopus CLI you can push your application packages for deployment as either Zip or NuGet packages, and manage your environments, deployments, projects, and workers.

The Octopus CLI can be used on Windows, Mac, Linux and Docker. For installation options and direct downloads, visit the [CLI Readme](https://github.com/OctopusDeploy/cli/blob/main/README.md).

:::hint
The Octopus CLI is built and maintained by the Octopus Deploy team, but it is also open source. You can [view the Octopus CLI project on GitHub](https://github.com/OctopusDeploy/cli), which leans heavily on the [go-octopusdeploy library](https://github.com/OctopusDeploy/go-octopusdeploy).
:::

## Commands {#octopusCommandLine-Commands}

` + "\n`octopus` supports the following commands:\n" +
	`
{{range .Pages}}
- **[{{.Title}}]({{.Filename}})**:  {{.Command.Short}}.{{end}}`
