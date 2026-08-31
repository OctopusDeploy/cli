package shared

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/cmd"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/OctopusDeploy/cli/pkg/question"
)

type Item struct {
	Label string
	Value string
	// Source distinguishes a detected value from one that was typed.
	Source string
	// A nil Edit means the value can only be changed by starting again.
	Edit func(context.Context) error
}

type Group struct {
	Title string
	Items []Item
}

// Review shows every setting, detected or chosen. Most are worked out rather
// than asked for, which is the point of the wizard, but it also means nobody
// sees them unless they are shown.
type Review struct {
	*cmd.Dependencies

	// Groups is rebuilt after every edit, so the screen reflects what changed.
	Groups func() []Group
	// Refresh recomputes anything derived from an edited value, such as a
	// namespace that follows the component's name.
	Refresh func() error
}

func (r *Review) Confirm(ctx context.Context) error {
	for {
		groups := r.Groups()
		PrintReview(r.Out, groups)

		const (
			install = "Install"
			change  = "Change a setting"
			cancel  = "Cancel"
		)

		answer := ""
		if err := r.Ask(&survey.Select{
			Message: "Ready to install?",
			Options: []string{install, change, cancel},
		}, &answer); err != nil {
			return err
		}

		switch answer {
		case install:
			return nil
		case cancel:
			return errors.New("install cancelled")
		}

		if err := r.editSetting(ctx, groups); err != nil {
			return err
		}
	}
}

func (r *Review) editSetting(ctx context.Context, groups []Group) error {
	type editable struct {
		label string
		edit  func(context.Context) error
	}

	var choices []editable
	for _, group := range groups {
		for _, item := range group.Items {
			if item.Edit != nil {
				choices = append(choices, editable{label: group.Title + ": " + item.Label, edit: item.Edit})
			}
		}
	}

	selected, err := question.SelectMap(r.Ask, "Which setting?", choices,
		func(e editable) string { return e.label })
	if err != nil {
		return err
	}

	if err := selected.edit(ctx); err != nil {
		return err
	}

	if r.Refresh == nil {
		return nil
	}
	return r.Refresh()
}

func PrintReview(out io.Writer, groups []Group) {
	width := 0
	for _, group := range groups {
		for _, item := range group.Items {
			if len(item.Label) > width {
				width = len(item.Label)
			}
		}
	}

	fmt.Fprintf(out, "\n%s\n", output.Bold("Review the installation"))
	for _, group := range groups {
		fmt.Fprintf(out, "\n  %s\n", output.Bold(group.Title))
		for _, item := range group.Items {
			fmt.Fprintf(out, "    %-*s  %s", width, item.Label, output.Cyan(item.Value))
			// An unset value has no source worth claiming.
			if item.Source != "" && !strings.HasPrefix(item.Value, "(") {
				fmt.Fprintf(out, "  %s", output.Dimf("(%s)", item.Source))
			}
			fmt.Fprintln(out)
		}
	}
	fmt.Fprintln(out)
}

// EditText edits a value in place, offering what is currently in effect as the
// default so pressing enter changes nothing.
func EditText(ask question.Asker, target *string, message string, current func() string) func(context.Context) error {
	return func(context.Context) error {
		value := current()
		if err := ask(&survey.Input{Message: message, Default: value}, &value); err != nil {
			return err
		}
		*target = strings.TrimSpace(value)
		return nil
	}
}

// EditConfirm edits a yes/no setting, starting from what is in effect.
func EditConfirm(ask question.Asker, target *bool, message, help string) func(context.Context) error {
	return func(context.Context) error {
		return ask(&survey.Confirm{Message: message, Default: *target, Help: help}, target)
	}
}

// DerivedOrSet describes where a value came from, for values the installer
// works out unless they were given explicitly.
func DerivedOrSet(explicit, derivedDescription string) string {
	if explicit != "" {
		return "set"
	}
	return derivedDescription
}

func OrNotSet(value string) string {
	return OrDefault(value, "(not set)")
}

func OrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func OrNone(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ", ")
}

// Masked keeps a credential out of the review screen and out of anything that
// scrapes the terminal.
func Masked(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	return "***"
}
