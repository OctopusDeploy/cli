package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	Title   string
	Command *cobra.Command
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

	if err := os.MkdirAll(*dir, 0644); err != nil {
		return err
	}

	if *website {
		if err := GenMarkdownTreeCustom(cmd, *dir); err != nil {
			return err
		}
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

func GenMarkdownTreeCustom(cmd *cobra.Command, dir string) error {
	for _, c := range cmd.Commands() {
		if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
			continue
		}
		if err := GenMarkdownTreeCustom(c, dir); err != nil {
			return err
		}
	}

	basename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
	filename := filepath.Join(dir, basename)
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	info := TemplateInformation{
		Title:   cmd.CommandPath(),
		Command: cmd,
	}

	if err := GenMarkdownCustom(cmd, f, info); err != nil {
		return err
	}
	return nil
}

const documentationTemplate = `---
title: {{.Title}}
description: {{.Command.Long}}
position:
---

{{.Command.Long}}
`
