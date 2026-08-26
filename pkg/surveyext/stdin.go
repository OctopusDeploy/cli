package surveyext

import (
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

const (
	keyEscape    = 0x1b
	keyDelete    = 0x7f
	keyBackspace = 0x08
	keyInterrupt = 0x03
	keyCR        = '\r'
	keyLF        = '\n'

	csiIntroducer = '['
	ss3Introducer = 'O'
)

// TranslatedStdin exists for Option/Alt+Backspace. Terminals send that as
// ESC DEL, which survey's rune reader rejects outright, ending the prompt and
// with it the whole command. Survey has no delete-word handling in its line
// editor either, so the sequence is rewritten into the right number of plain
// backspaces.
//
// It embeds the real file because survey puts the terminal into raw mode with
// an ioctl against Fd().
type TranslatedStdin struct {
	file *os.File

	// line models what has been typed since the prompt last accepted a line,
	// so a word deletion knows how far back to reach.
	line []rune
	// lineUnknown records that something happened this line the model does not
	// follow. Deleting a guessed number of characters would corrupt the input,
	// so word deletion falls back to removing one character.
	lineUnknown bool
	// pendingEscape holds back a trailing ESC: a terminal may split an escape
	// sequence across reads, and an ESC released early reaches survey as the
	// malformed sequence this type exists to prevent. The cost is that a bare
	// Escape keypress is not seen until the next byte arrives, which only
	// happens when a read ends exactly on an ESC.
	pendingEscape bool
	// overflow holds translated bytes that did not fit in the caller's buffer,
	// since one keypress can expand into several backspaces.
	overflow []byte
}

func NewTranslatedStdin(file *os.File) *TranslatedStdin {
	return &TranslatedStdin{file: file}
}

func (s *TranslatedStdin) Fd() uintptr { return s.file.Fd() }

func (s *TranslatedStdin) File() *os.File { return s.file }

// TerminalFile unwraps a reader to the terminal behind it. Anything handing
// stdin to a child process needs the real file: os/exec only passes a
// descriptor straight through when given an *os.File, and otherwise copies
// through a pipe, leaving a full-screen editor without a terminal to drive.
func TerminalFile(reader io.Reader) (*os.File, bool) {
	switch r := reader.(type) {
	case *os.File:
		return r, true
	case *TranslatedStdin:
		return r.File(), true
	default:
		return nil, false
	}
}

func (s *TranslatedStdin) Read(p []byte) (int, error) {
	if len(s.overflow) > 0 {
		n := copy(p, s.overflow)
		s.overflow = s.overflow[n:]
		return n, nil
	}

	n, err := s.file.Read(p)
	if n <= 0 {
		if s.pendingEscape && len(p) > 0 {
			s.pendingEscape = false
			p[0] = keyEscape
			return 1, err
		}
		return n, err
	}

	translated := s.translate(p[:n])

	copied := copy(p, translated)
	if copied < len(translated) {
		s.overflow = append(s.overflow, translated[copied:]...)
	}
	return copied, err
}

func (s *TranslatedStdin) translate(in []byte) []byte {
	out := make([]byte, 0, len(in))

	for i := 0; i < len(in); {
		b := in[i]

		if s.pendingEscape {
			s.pendingEscape = false
			if consumed, replacement, ok := s.escapeSequence(in[i:]); ok {
				out = append(out, replacement...)
				i += consumed
				continue
			}
			out = append(out, keyEscape)
		}

		if b == keyEscape {
			if i == len(in)-1 {
				s.pendingEscape = true
				i++
				continue
			}
			if consumed, replacement, ok := s.escapeSequence(in[i+1:]); ok {
				out = append(out, replacement...)
				i += 1 + consumed
				continue
			}
			s.lineUnknown = true
			out = append(out, b)
			i++
			continue
		}

		r, size := utf8.DecodeRune(in[i:])
		s.observe(r)
		out = append(out, in[i:i+size]...)
		i += size
	}

	return out
}

func (s *TranslatedStdin) escapeSequence(rest []byte) (consumed int, replacement []byte, ok bool) {
	if len(rest) == 0 {
		return 0, nil, false
	}

	switch rest[0] {
	case keyDelete, keyBackspace:
		return 1, s.deleteWord(), true
	case csiIntroducer:
		return s.controlSequence(rest)
	case ss3Introducer:
		if len(rest) < 2 {
			return 0, nil, false
		}
		s.lineUnknown = true
		return 2, append([]byte{keyEscape}, rest[:2]...), true
	default:
		return 0, nil, false
	}
}

// controlSequence consumes a CSI sequence whole so its payload never reaches
// the line model as typed characters. The terminal's own replies arrive on the
// input stream: survey asks for the cursor position on every render, and the
// answer comes back as ESC [ row ; col R.
func (s *TranslatedStdin) controlSequence(rest []byte) (consumed int, replacement []byte, ok bool) {
	end := 1
	for end < len(rest) && !isCSIFinalByte(rest[end]) {
		end++
	}
	if end == len(rest) {
		// Split across reads; leave it for survey rather than half-consuming it.
		s.lineUnknown = true
		return 0, nil, false
	}

	if movesCursor(rest[end]) {
		s.lineUnknown = true
	}

	consumed = end + 1
	return consumed, append([]byte{keyEscape}, rest[:consumed]...), true
}

func isCSIFinalByte(b byte) bool { return b >= 0x40 && b <= 0x7e }

func movesCursor(final byte) bool {
	switch final {
	case 'R', 'c', 'n': // replies from the terminal, not keys the user pressed
		return false
	default:
		return true
	}
}

func (s *TranslatedStdin) deleteWord() []byte {
	if s.lineUnknown || len(s.line) == 0 {
		return []byte{keyDelete}
	}

	count := 0
	for len(s.line)-count > 0 && unicode.IsSpace(s.line[len(s.line)-count-1]) {
		count++
	}
	for len(s.line)-count > 0 && !unicode.IsSpace(s.line[len(s.line)-count-1]) {
		count++
	}

	s.line = s.line[:len(s.line)-count]

	backspaces := make([]byte, count)
	for i := range backspaces {
		backspaces[i] = keyDelete
	}
	return backspaces
}

func (s *TranslatedStdin) observe(r rune) {
	switch {
	case r == keyCR || r == keyLF || r == keyInterrupt:
		s.line = s.line[:0]
		s.lineUnknown = false
	case r == keyDelete || r == keyBackspace:
		if len(s.line) > 0 {
			s.line = s.line[:len(s.line)-1]
		}
	case unicode.IsPrint(r):
		s.line = append(s.line, r)
	default:
		s.lineUnknown = true
	}
}
