package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	version "github.com/OctopusDeploy/cli"
	"github.com/OctopusDeploy/cli/pkg/apiclient"
	"github.com/OctopusDeploy/cli/pkg/cmd/root"
	"github.com/OctopusDeploy/cli/pkg/factory"
	"github.com/OctopusDeploy/cli/pkg/question"
	"github.com/briandowns/spinner"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/pflag"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

const fmTemplate = `---
date: %s
title: "%s"
slug: %s
url: %s
---
`

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
	cmd.InitDefaultHelpCmd()

	if err := os.MkdirAll(*dir, 0755); err != nil {
		return err
	}

	filePrepender := func(filename string) string {
		now := time.Now().Format(time.RFC3339)
		name := filepath.Base(filename)
		base := strings.TrimSuffix(name, path.Ext(name))
		url := "/commands/" + strings.ToLower(base) + "/"
		return fmt.Sprintf(fmTemplate, now, strings.Replace(base, "_", " ", -1), base, url)
	}

	linkHandler := func(name string) string {
		base := strings.TrimSuffix(name, path.Ext(name))
		return "/commands/" + strings.ToLower(base) + "/"
	}

	if *website {
		if err := doc.GenMarkdownTreeCustom(cmd, *dir, filePrepender, linkHandler); err != nil {
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
