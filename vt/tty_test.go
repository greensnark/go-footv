package vt

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

type WriteCase struct {
	text   string
	checks []StateTest
}

type StateTest interface {
	Test(*Tty) string
}

type CheckFn func(*Tty) string

func (s CheckFn) Test(tty *Tty) string { return s(tty) }

func cur(pt Pt) CheckFn {
	return func(tty *Tty) string {
		if tty.Cursor != pt {
			return fmt.Sprintf("expected cursor=%s, got %s",
				pt.String(), tty.Cursor.String())
		}
		return ""
	}
}

func txt(pt Pt, text string) CheckFn {
	return func(tty *Tty) string {
		actual := tty.TextAtN(pt, len(text))
		if actual != text {
			return fmt.Sprintf("expected textAt%s==%#v, got %#v",
				pt, text, actual)
		}
		return ""
	}
}

func checks(tests ...StateTest) []StateTest {
	return tests
}

var writeTests = []WriteCase{
	{"Hello, world", checks(
		cur(Pt{12, 0}),
		txt(Pt{}, "Hello, world"),
	)},
	{"Hi\nthere", checks(
		cur(Pt{5, 1}),
		txt(Pt{}, "Hi "),
		txt(Pt{Y: 1}, "there "),
	)},
	{"Hello\033[2J", checks(cur(Pt{5, 0}), txt(Pt{}, " "))},
	{"Hello\033[2J\033[H", checks(cur(Pt{}), txt(Pt{}, " "))},
}

func TestWrite(t *testing.T) {
	term := New()
	for _, test := range writeTests {
		term.Reset()
		term.WriteString(test.text)
		for _, check := range test.checks {
			if res := check.Test(term); res != "" {
				t.Errorf("test write %#v failed: %s", test.text, res)
			}
		}
	}
}

type testFile struct {
	name           string
	input          string
	expectedOutput string
}

func unescape(in []byte) string {
	res := &bytes.Buffer{}
	escaped := false
	for _, b := range in {
		// Skip unprintable characters.
		if b < 32 {
			continue
		}
		if escaped {
			switch b {
			case 'a':
				res.WriteByte(7)
			case 'b':
				res.WriteByte(8)
			case 'f':
				res.WriteByte(12)
			case '\\':
				res.WriteByte('\\')
			case 'e':
				res.WriteByte(0x1b)
			case 'r':
				res.WriteByte('\r')
			case 'n':
				res.WriteByte('\n')
			case 't':
				res.WriteByte('\t')
			case '\'':
				res.WriteByte('\'')
			default:
				res.WriteByte('\\')
				res.WriteByte(b)
			}
			escaped = false
		} else if b == '\\' {
			escaped = true
		} else {
			res.WriteByte(b)
		}
	}
	return res.String()
}

func (t *testFile) ExpectedOutput() string { return t.expectedOutput }
func (t *testFile) UnescapedInput() string { return t.input }
func (t *testFile) String() string         { return t.name }

func newTestFile(in string, out string) *testFile {
	rawInput, err := ioutil.ReadFile(in)
	if err != nil {
		panic(err)
	}
	expectedOutput, err := ioutil.ReadFile(out)
	if err != nil {
		panic(err)
	}
	return &testFile{
		name:           in,
		input:          unescape(rawInput),
		expectedOutput: string(expectedOutput),
	}
}

func getTestBlobs() []*testFile {
	files, err := ioutil.ReadDir("vt.in")
	if err != nil {
		panic(err)
	}
	res := make([]*testFile, len(files))
	for i, file := range files {
		in := path.Join("vt.in", file.Name())
		out := path.Join("vt.out", file.Name())
		res[i] = newTestFile(in, out)
	}
	return res
}

func TestBlobs(t *testing.T) {
	term := NewSz(Pt{20, 5})
	term.Debug = true
	for _, file := range getTestBlobs() {
		fmt.Fprintln(os.Stderr, "Testing ", file.String())
		term.Reset()
		term.WriteString(file.UnescapedInput())
		if out := term.DebugDump(); out != file.ExpectedOutput() {
			t.Errorf("unexpected output for %s:\n%s\nexpected:\n%s",
				file.String(), out, file.ExpectedOutput())
		}
	}
}
