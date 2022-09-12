package surveyext

import (
	"errors"
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	surveyCore "github.com/AlecAivazis/survey/v2/core"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/OctopusDeploy/cli/pkg/output"
	"github.com/mgutz/ansi"
	"strconv"
	"strings"
	"time"
)

/*
DatePicker is a prompt that presents a date/time picker. Response type is a time.Time

	selectedTime := time.Time{}
	prompt := &surveyext.DatePicker {
		Message: "Choose a date and time:",
		Default: time.Now(),
	}
	survey.AskOne(prompt, &selectedTime)
*/
type DatePicker struct {
	survey.Renderer
	Message         string
	Default         time.Time
	Min             time.Time
	Max             time.Time
	Help            string
	AnswerFormatter func(time.Time, time.Time) string // first parameter is 'now', second parameter is the user's selected time
	OverrideNow     time.Time                         // for unit testing; lets you override the definition of 'now'

	selectedComponent componentIdx
	runeBuffer        []rune
	showingHelp       bool
}

type DatePickerTemplateData struct {
	DatePicker
	RawInput          string // this is full of ansi escape sequences... It'd be nice to have survey's template thing render this, TODO attempt that later
	Answer            string
	ShowAnswer        bool
	ShowHelp          bool
	SelectedComponent componentIdx
	Config            *survey.PromptConfig
}

var DatePickerQuestionTemplate = `
{{- if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default+hb"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "cyan"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{ .RawInput }}{{color "reset"}}{{"\n"}}
{{- end}}`

// DatePickerAnswer exists to workaround a survey bug (unintented code path?) where if the answer is a struct it
// thinks you're asking multiple questions and collecting the answers into struct fields.
// If you do this:
//
//	var answer time.Time
//	err = asker(&surveyext.DatePicker{ Message: "When?" }, &answer)
//
// then the code in survey Write.go sees that the answer is a struct, and tries to go putting
// the response value into a named field on that struct (which doesn't exist).
// Workaround is to have a response-holder structure that implements survey.core.Settable
type DatePickerAnswer struct {
	Time time.Time
}

var _ surveyCore.Settable = (*DatePickerAnswer)(nil)

func defaultAnswerFormatter(_ time.Time, t time.Time) string { return t.String() }

func (a *DatePickerAnswer) WriteAnswer(_ string, value interface{}) error {
	if v, ok := value.(time.Time); ok {
		a.Time = v
		return nil
	} else {
		return errors.New("DatePickerAnswer.WriteAnswer received non-time value")
	}
}

func (d *DatePicker) Cleanup(config *survey.PromptConfig, val interface{}) error {
	t := val.(time.Time)
	d.selectedComponent = cmpNone

	answerFormatter := d.AnswerFormatter
	if answerFormatter == nil {
		answerFormatter = defaultAnswerFormatter
	}
	err := d.Render(
		DatePickerQuestionTemplate,
		DatePickerTemplateData{
			DatePicker: *d,
			ShowAnswer: true,
			ShowHelp:   d.showingHelp,
			Answer:     answerFormatter(d.OverrideNow, t),
			Config:     config,
		})
	return err
}

func (d *DatePicker) Error(*survey.PromptConfig, error) error {
	return nil // do nothing; our prompt loop is self-contained
}

var invertedCyan = ansi.ColorFunc("black:cyan+h")
var invertedBlackWhite = ansi.ColorFunc("black:white")

func invertedCyanf(s string, args ...any) string {
	if !output.IsColorEnabled {
		// Note: if this does end up on a non-interactive terminal it will print escape codes to the screen,
		// however if we don't do this then some person who's enabled NO_COLOR for accessibility reasons
		// won't be able to use the control
		return invertedBlackWhite(fmt.Sprintf(s, args...))
	}
	return invertedCyan(fmt.Sprintf(s, args...))
}

func (d *DatePicker) Now() time.Time {
	if !d.OverrideNow.IsZero() {
		return d.OverrideNow
	} else {
		return time.Now()
	}
}

func (d *DatePicker) Prompt(config *survey.PromptConfig) (interface{}, error) {
	var t time.Time

	min := stripMilliseconds(d.Min)
	max := stripMilliseconds(d.Max)

	// choose the initial value. Use Default if supplied, otherwise use time.Now (constrained within Min/Max)
	if d.Default.IsZero() {
		t = clamp(stripMilliseconds(d.Now()), min, max)
	} else {
		t = clamp(stripMilliseconds(d.Default), min, max)
	}

	cursor := d.NewCursor()
	_ = cursor.Hide()
	defer func() { _ = cursor.Show() }()

	d.selectedComponent = cmpYear

	done := false
	for !done {
		err := d.Render(
			DatePickerQuestionTemplate,
			DatePickerTemplateData{
				DatePicker: *d,
				RawInput:   d.printTimeComponents(t),
				ShowHelp:   d.showingHelp,
				Config:     config,
			})
		if err != nil {
			return time.Time{}, err
		}

		rr := d.NewRuneReader()
		_ = rr.SetTermMode()
		// remember to put _ = rr.RestoreTermMode() on every early return. can't use defer because we're in a loop

		// read a single rune at a time
		r, _, err := rr.ReadRune()
		if err != nil {
			_ = rr.RestoreTermMode()
			return time.Time{}, err
		}

		var helpRune rune = 0
		if config.HelpInput != "" {
			for _, r := range config.HelpInput {
				helpRune = r
				break
			}
		}

		switch r {
		case terminal.KeyInterrupt:
			_ = rr.RestoreTermMode()
			return time.Time{}, terminal.InterruptErr
		case terminal.KeyEndTransmission, terminal.KeyEnter, '\n':
			t = clamp(d.commitRuneBuffer(t), min, max)
			// if we don't re-print on exiting the loop it looks weird if we have uncommitted text
			// let it fall through and exit the loop after it's printed
			done = true
		case helpRune:
			if d.Help != "" {
				d.showingHelp = true
			}
		case terminal.KeyArrowUp:
			t = clamp(incrementComponent(d.commitRuneBuffer(t), d.selectedComponent), min, max)
		case terminal.KeyArrowDown:
			t = clamp(decrementComponent(d.commitRuneBuffer(t), d.selectedComponent), min, max)
		case terminal.KeyArrowLeft:
			t = clamp(d.commitRuneBuffer(t), min, max)
			if d.selectedComponent > 0 {
				d.selectedComponent--
			}
		case terminal.KeyArrowRight, terminal.KeyTab, ':', ' ', '-':
			t = clamp(d.commitRuneBuffer(t), min, max)
			if d.selectedComponent < cmpLast {
				d.selectedComponent++
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			t = clamp(d.consumeRune(r, t), min, max)
		}

		_ = rr.RestoreTermMode()
		if done { // last extra print before we exit the loop
			d.selectedComponent = cmpNone
			err = d.Render(
				DatePickerQuestionTemplate,
				DatePickerTemplateData{
					DatePicker: *d,
					RawInput:   d.printTimeComponents(t), // this overtypes any background color format etc
					ShowAnswer: false,
					ShowHelp:   d.showingHelp,
					Config:     config,
				})
			if err != nil {
				return time.Time{}, err
			}
		}
	}
	return t, nil
}

func (d *DatePicker) commitRuneBuffer(t time.Time) time.Time {
	if len(d.runeBuffer) == 0 {
		return t // nothing to be done here
	}
	intVal, err := strconv.Atoi(string(d.runeBuffer))
	d.runeBuffer = d.runeBuffer[:0] // always clear the buffer
	if err == nil {
		return setComponent(t, d.selectedComponent, intVal)
	} // if atoi somehow failed, just ignore it. Should never happen
	return t
}

func (d *DatePicker) consumeRune(r rune, t time.Time) time.Time {
	d.runeBuffer = append(d.runeBuffer, r)

	// this int parse is just for range-checking, we don't commit here unless commitRuneBuffer is called
	intVal, err := strconv.Atoi(string(d.runeBuffer))
	if err != nil {
		// reset the buffer and bail
		d.runeBuffer = d.runeBuffer[:0]
		return t
	}
	// ignore things that are too big (e.g. they type 97 into 'seconds')
	// also clear the buffer if they've typed all the characters
	maxVal := 9999
	maxBufLen := 4
	switch d.selectedComponent {
	case cmpYear:
		maxVal, maxBufLen = 9999, 4
	case cmpMonth:
		maxVal, maxBufLen = 12, 2
	case cmpDay:
		maxVal, maxBufLen = 31, 2 // not smart enough to handle february or short months
	case cmpHour:
		maxVal, maxBufLen = 24, 2 // no am/pm
	case cmpMinute, cmpSecond:
		maxVal, maxBufLen = 59, 2
	default:
		maxVal, maxBufLen = 0, 0 // force reset if we somehow end up here
	}

	if intVal > maxVal {
		// e.g. they've typed a '3' in the Month column, then go to type a second character and end up with '32'
		d.runeBuffer = d.runeBuffer[:0]
	} else if len(d.runeBuffer) >= maxBufLen {
		// intval is good, and we want to use it
		t = d.commitRuneBuffer(t)
		// auto-move to next field
		if d.selectedComponent < cmpLast {
			d.selectedComponent++
		}
	}
	return t
}

func (d *DatePicker) printTimeComponents(t time.Time) string {
	if d.selectedComponent < cmpNone || d.selectedComponent > cmpLast {
		panic("selectedComponent out of allowable range") // this should never happen; if it does we wrote a bug and we should fix it
	}

	// yyyy/mm/dd hh:mm:ss
	var printers [cmpLast + 1]func(fmt string, args ...any) string
	printers[cmpYear] = output.Cyanf
	printers[cmpMonth] = output.Cyanf
	printers[cmpDay] = output.Cyanf
	printers[cmpHour] = output.Cyanf
	printers[cmpMinute] = output.Cyanf
	printers[cmpSecond] = output.Cyanf

	if d.selectedComponent >= 0 {
		if len(d.runeBuffer) > 0 {
			// we have some uncommitted text in the buffer, print it over the top of the entry.
			// this allows us to hold the appearance of bad input without modifying the actual
			// underlying datetime value until we 'commit'.
			// If we don't do this then when focus is on the month field and you type "02" it goes wrong
			// because it first sees "0" which is not a valid month, and tries to parse it (the year goes backwards unintentionally)
			printers[d.selectedComponent] = func(fmt string, _ ...any) string {
				// turn "%02d" into "%2s" but preserve the number
				tmp := strings.Replace(fmt, "d", "s", 1)
				tmp = strings.Replace(tmp, "0", "", 1)
				return invertedCyanf(tmp, string(d.runeBuffer))
			}
		} else {
			printers[d.selectedComponent] = invertedCyanf
		}
	}

	_, tzOffset := t.Zone()
	tzoSign := "+"
	if tzOffset < 0 {
		tzoSign = "-"
	}

	tzoHours := tzOffset / 3600
	tzoMins := (tzOffset % 3600) / 60

	return fmt.Sprintf("%s/%s/%s %s:%s:%s  GMT %s%02d:%02d",
		printers[cmpYear]("%04d", t.Year()),
		printers[cmpMonth]("%02d", t.Month()),
		printers[cmpDay]("%02d", t.Day()),
		printers[cmpHour]("%02d", t.Hour()),
		printers[cmpMinute]("%02d", t.Minute()),
		printers[cmpSecond]("%02d", t.Second()),
		tzoSign,
		tzoHours,
		tzoMins,
	)
}

type componentIdx int

const (
	cmpNone = componentIdx(-1)
	cmpLast = 5

	cmpYear   = componentIdx(0)
	cmpMonth  = componentIdx(1)
	cmpDay    = componentIdx(2)
	cmpHour   = componentIdx(3)
	cmpMinute = componentIdx(4)
	cmpSecond = componentIdx(5)
)

func addComponent(t time.Time, cmp componentIdx, n int) time.Time {
	switch cmp {
	case cmpYear:
		return t.AddDate(n, 0, 0)
	case cmpMonth:
		return t.AddDate(0, n, 0)
	case cmpDay:
		return t.AddDate(0, 0, n)
	case cmpHour:
		return t.Add(time.Duration(n) * time.Hour)
	case cmpMinute:
		return t.Add(time.Duration(n) * time.Minute)
	case cmpSecond:
		return t.Add(time.Duration(n) * time.Second)
	default:
		panic("incrementComponent out of allowable range")
	}
}

func stripMilliseconds(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, t.Location())
}

func clamp(t time.Time, min time.Time, max time.Time) time.Time {
	candidate := t
	if !min.IsZero() && candidate.Before(min) {
		candidate = min
	}
	if !max.IsZero() && candidate.After(max) {
		candidate = max
	}
	return candidate
}

func incrementComponent(t time.Time, component componentIdx) time.Time {
	return addComponent(t, component, 1)
}

func decrementComponent(t time.Time, component componentIdx) time.Time {
	return addComponent(t, component, -1)
}

func setComponent(t time.Time, component componentIdx, value int) time.Time {
	y := t.Year()
	M := t.Month()
	d := t.Day()
	h := t.Hour()
	m := t.Minute()
	s := t.Second()

	switch component {
	case cmpNone:
		break // special case to allow clearing of milliseconds on entry to the date picker
	case cmpYear:
		y = value
	case cmpMonth:
		M = time.Month(value)
	case cmpDay:
		d = value
	case cmpHour:
		h = value
	case cmpMinute:
		m = value
	case cmpSecond:
		s = value
	}
	return time.Date(y, M, d, h, m, s, 0, t.Location())
}
