package ttyrec

import (
	"os"
	"testing"
	"time"
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

func TestReadTtyrec(t *testing.T) {
	file, err := os.Open("test/test.ttyrec")
	if err != nil {
		panic(err)
	}
	ttr := Reader(file)

	frames := []wantFrame{
		{tm("2015-03-04T02:31:12.467019Z"), 0x96},
		{tm("2015-03-04T02:31:13.629597Z"), 0x20},
	}

	for i, f := range frames {
		frame, err := ttr.ReadFrame()
		if err != nil {
			panic(err)
		}
		if frame.Time != f.Time {
			t.Errorf("frame:%d time: %s, want %s", i, frame.Time, f.Time)
		}
		if len(frame.Body) != f.Size {
			t.Errorf("frame:%d size %d, want %d", i, len(frame.Body), f.Size)
		}
	}
}
