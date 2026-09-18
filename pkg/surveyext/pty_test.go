package surveyext_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/OctopusDeploy/cli/pkg/surveyext"
	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cursorPositionRequest is the escape sequence survey uses to locate the
// cursor. A real terminal answers it; a bare pty has nobody to, so a test
// driving survey through one has to answer on the terminal's behalf or every
// prompt hangs.
var cursorPositionRequest = []byte("\x1b[6n")

type fakeTerminal struct {
	master *os.File
	slave  *os.File

	mu     sync.Mutex
	output bytes.Buffer
}

func newFakeTerminal(t *testing.T) *fakeTerminal {
	t.Helper()

	master, slave, err := pty.Open()
	require.NoError(t, err)

	term := &fakeTerminal{master: master, slave: slave}
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})

	go term.pump()
	return term
}

func (f *fakeTerminal) pump() {
	buf := make([]byte, 1024)
	for {
		n, err := f.master.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			f.mu.Lock()
			f.output.Write(chunk)
			f.mu.Unlock()

			for i := 0; i < bytes.Count(chunk, cursorPositionRequest); i++ {
				_, _ = f.master.Write([]byte("\x1b[1;1R"))
			}
		}
		if err != nil {
			return
		}
	}
}

func (f *fakeTerminal) typeKeys(t *testing.T, keys string) {
	t.Helper()
	// Let the prompt render before typing at it.
	require.Eventually(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.output.Len() > 0
	}, 5*time.Second, 20*time.Millisecond, "the prompt never rendered")

	_, err := f.master.Write([]byte(keys))
	require.NoError(t, err)
}

func (f *fakeTerminal) ask(translate bool) (<-chan string, <-chan error) {
	answer := make(chan string, 1)
	errs := make(chan error, 1)

	var in survey.AskOpt
	if translate {
		in = survey.WithStdio(surveyext.NewTranslatedStdin(f.slave), f.slave, io.Discard)
	} else {
		in = survey.WithStdio(f.slave, f.slave, io.Discard)
	}

	go func() {
		var response string
		if err := survey.AskOne(&survey.Input{Message: "Name"}, &response, in); err != nil {
			errs <- err
			return
		}
		answer <- response
	}()

	return answer, errs
}

// This is the reported bug: Option+Backspace sends ESC DEL, survey's rune
// reader rejects it, and the prompt aborts - taking the whole command with it.
func TestAskOne_UntranslatedOptionBackspaceIsRejectedBySurvey(t *testing.T) {
	term := newFakeTerminal(t)
	_, errs := term.ask(false)

	term.typeKeys(t, "liam mackie\x1b\x7f\r")

	select {
	case err := <-errs:
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "unexpected escape sequence from terminal"),
			"expected survey to reject the sequence, got: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the prompt")
	}
}

// Translated, the same keypress does what it does everywhere else: delete the
// previous word.
func TestAskOne_OptionBackspaceDeletesAWord(t *testing.T) {
	term := newFakeTerminal(t)
	answer, errs := term.ask(true)

	term.typeKeys(t, "liam mackie\x1b\x7f\r")

	select {
	case got := <-answer:
		assert.Equal(t, "liam ", got)
	case err := <-errs:
		t.Fatalf("prompt aborted: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the prompt")
	}
}

func TestAskOne_NormalInputIsUnaffected(t *testing.T) {
	term := newFakeTerminal(t)
	answer, errs := term.ask(true)

	term.typeKeys(t, "liam-mackie\r")

	select {
	case got := <-answer:
		assert.Equal(t, "liam-mackie", got)
	case err := <-errs:
		t.Fatalf("prompt aborted: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the prompt")
	}
}
