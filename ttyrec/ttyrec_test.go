package ttyrec

import (
	"testing"
	"time"

	"github.com/greensnark/go-footv/compfile"
)

type wantFrame struct {
	time.Time
	Size int
}

func tm(text string) time.Time {
	t, err := time.Parse(time.RFC3339, text)
	if err != nil {
		panic(err)
	}
	return t
}

var testFrames = []wantFrame{
	{tm("2015-03-04T02:31:12.467019Z"), 0x96},
	{tm("2015-03-04T02:31:13.629597Z"), 0x20},
}

var exts = []string{"", ".gz", ".bz2"}

func TestReadTtyrec(t *testing.T) {
	baseFile := "test/test.ttyrec"
	for _, ext := range exts {
		filename := baseFile + ext
		file, err := compfile.Open(filename)
		if err != nil {
			panic(err)
		}

		defer file.Close()
		ttr := Reader(file)

		for i, f := range testFrames {
			frame, err := ttr.ReadFrame()
			if err != nil {
				panic(err)
			}
			if frame.Time != f.Time {
				t.Errorf("[%s] frame:%d time: %s, want %s", filename, i,
					frame.Time, f.Time)
			}
			if len(frame.Body) != f.Size {
				t.Errorf("[%s] frame:%d size %d, want %d", filename, i,
					len(frame.Body), f.Size)
			}
		}
	}
}
