package surveyext

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	optionBackspace = "\x1b\x7f"
	del             = "\x7f"
)

// translateAll includes anything the translator holds back as overflow.
func translateAll(t *testing.T, chunks ...string) string {
	t.Helper()

	s := &TranslatedStdin{}
	var out strings.Builder
	for _, chunk := range chunks {
		out.Write(s.translate([]byte(chunk)))
	}
	return out.String()
}

// Survey's line editor only understands plain backspaces, so a word deletion
// has to become the right number of those.
func TestTranslate_OptionBackspaceDeletesAWord(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"one word", "liam" + optionBackspace, "liam" + strings.Repeat(del, 4)},
		{"last of two words", "liam mackie" + optionBackspace, "liam mackie" + strings.Repeat(del, 6)},
		{"trailing space is consumed with the word", "liam mackie " + optionBackspace, "liam mackie " + strings.Repeat(del, 7)},
		{"hyphens are part of a word", "my-gateway" + optionBackspace, "my-gateway" + strings.Repeat(del, 10)},
		{"nothing typed yet", optionBackspace, del},
		{"the backspace variant behaves the same", "liam\x1b\x08", "liam" + strings.Repeat(del, 4)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, translateAll(t, tt.input))
		})
	}
}

func TestTranslate_RepeatedWordDeletes(t *testing.T) {
	got := translateAll(t, "octopus argo gateway"+optionBackspace+optionBackspace)
	assert.Equal(t, "octopus argo gateway"+strings.Repeat(del, 7)+strings.Repeat(del, 5), got)
}

// Backspaces the user types themselves have to move the model too, or the next
// word deletion reaches too far.
func TestTranslate_TracksManualBackspaces(t *testing.T) {
	got := translateAll(t, "liam mackie"+del+del+optionBackspace)
	assert.Equal(t, "liam mackie"+del+del+strings.Repeat(del, 4), got, "mack remains, so four backspaces")
}

func TestTranslate_ResetsOnNewline(t *testing.T) {
	got := translateAll(t, "first answer\rsecond"+optionBackspace)
	assert.Equal(t, "first answer\rsecond"+strings.Repeat(del, 6), got)
}

// A cursor movement puts the model out of step with the real line. Guessing a
// word boundary then would eat text the user meant to keep, so fall back to
// removing a single character.
func TestTranslate_FallsBackAfterCursorMovement(t *testing.T) {
	arrowLeft := "\x1b[D"
	got := translateAll(t, "liam mackie"+arrowLeft+optionBackspace)
	assert.Equal(t, "liam mackie"+arrowLeft+del, got)
}

func TestTranslate_FallsBackAfterAnUnmodelledControlKey(t *testing.T) {
	ctrlA := "\x01"
	got := translateAll(t, "liam mackie"+ctrlA+optionBackspace)
	assert.Equal(t, "liam mackie"+ctrlA+del, got)
}

func TestTranslate_PassesThroughSurveysOwnSequences(t *testing.T) {
	// A trailing character is added so nothing is held back waiting to see what
	// follows the escape.
	for _, seq := range []string{"\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D", "\x1bOA", "\x1b[3~", "\x1b[1;1R"} {
		assert.Equal(t, seq+"x", translateAll(t, seq+"x"), "%q must reach survey unchanged", seq)
	}
}

// Survey asks the terminal for the cursor position on every render, and the
// answer arrives on the input stream. Treating that reply as typing would
// corrupt the line model and make word deletion fall back to one character.
func TestTranslate_CursorPositionReplyDoesNotDisturbTheLineModel(t *testing.T) {
	cursorReply := "\x1b[1;1R"
	got := translateAll(t, cursorReply+"liam mackie"+optionBackspace)
	assert.Equal(t, cursorReply+"liam mackie"+strings.Repeat(del, 6), got)
}

func TestTranslate_LeavesOrdinaryTypingAlone(t *testing.T) {
	for _, input := range []string{"liam-mackie", "Production", "grpc://host:8443", ""} {
		assert.Equal(t, input, translateAll(t, input))
	}
}

// A terminal may split an escape sequence across reads. Releasing the ESC
// before knowing what follows would hand survey the very sequence it rejects.
func TestTranslate_HandlesSplitEscapeSequences(t *testing.T) {
	s := &TranslatedStdin{}
	first := string(s.translate([]byte("liam mackie\x1b")))
	second := string(s.translate([]byte("\x7f")))

	assert.Equal(t, "liam mackie", first, "the ESC is held until what follows it is known")
	assert.Equal(t, strings.Repeat(del, 6), second)
	assert.NotContains(t, first+second, "\x1b", "survey must never see the sequence it cannot parse")
}

func TestTranslate_ReleasesAHeldEscapeWhenItWasNotASequence(t *testing.T) {
	s := &TranslatedStdin{}
	first := string(s.translate([]byte("\x1b")))
	second := string(s.translate([]byte("a")))

	assert.Empty(t, first)
	assert.Equal(t, "\x1ba", second)
}

func TestTranslate_MultiByteRunesCountAsOneCharacter(t *testing.T) {
	got := translateAll(t, "café"+optionBackspace)
	assert.Equal(t, "café"+strings.Repeat(del, 4), got, "é is one character to delete, not two bytes")
}

// One keypress can expand past the caller's buffer, and no keystroke may be
// dropped when it does.
func TestRead_HoldsOverflowForTheNextRead(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	defer reader.Close()

	go func() {
		defer writer.Close()
		_, _ = writer.Write([]byte("liam mackie" + optionBackspace))
	}()

	stdin := NewTranslatedStdin(reader)

	var got bytes.Buffer
	buf := make([]byte, 4)
	for got.Len() < len("liam mackie")+6 {
		n, err := stdin.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			break
		}
	}

	assert.Equal(t, "liam mackie"+strings.Repeat(del, 6), got.String())
}

func TestTerminalFile_Unwraps(t *testing.T) {
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	defer reader.Close()
	defer writer.Close()

	got, ok := TerminalFile(NewTranslatedStdin(reader))
	assert.True(t, ok)
	assert.Same(t, reader, got)

	got, ok = TerminalFile(reader)
	assert.True(t, ok)
	assert.Same(t, reader, got)

	_, ok = TerminalFile(bytes.NewBufferString("not a terminal"))
	assert.False(t, ok)
}
