package ttyrec

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

type TReader struct {
	r        io.Reader
	hbuf     []byte
	framebuf []byte
	frame    *Frame
}

const HeaderSize = 12

// Don't try to read ttyrec frames larger than this
const MaxFrameSizeBytes = 1e6

// Default size of frame body
const FrameBufDefault = 16384

func Reader(r io.Reader) *TReader {
	return &TReader{
		r:    r,
		hbuf: make([]byte, HeaderSize),
	}
}

type header struct {
	sec    uint32
	usec   uint32
	length uint32
}

func (h header) Time() time.Time {
	return time.Unix(int64(h.sec), int64(h.usec)*1e3).UTC()
}

type Frame struct {
	time.Time
	Body []byte
}

func (t *TReader) readHeader() (header, error) {
	if _, err := io.ReadFull(t.r, t.hbuf); err != nil {
		return header{}, err
	}
	return header{
		sec:    binary.LittleEndian.Uint32(t.hbuf[0:4]),
		usec:   binary.LittleEndian.Uint32(t.hbuf[4:8]),
		length: binary.LittleEndian.Uint32(t.hbuf[8:12]),
	}, nil
}

func (t *TReader) frameBufSize(min uint32) uint32 {
	if min > FrameBufDefault {
		return min
	}
	return FrameBufDefault
}

func (t *TReader) readFrameBody(hdr header) (*Frame, error) {
	if hdr.length > MaxFrameSizeBytes {
		return nil, fmt.Errorf("ttyrec frame too large: %u", hdr.length)
	}
	if int(hdr.length) > cap(t.framebuf) {
		t.framebuf = make([]byte, hdr.length, t.frameBufSize(hdr.length))
	} else {
		t.framebuf = t.framebuf[0:hdr.length]
	}
	if _, err := io.ReadFull(t.r, t.framebuf); err != nil {
		return nil, err
	}
	if t.frame == nil {
		t.frame = &Frame{}
	}
	t.frame.Time = hdr.Time()
	t.frame.Body = t.framebuf
	return t.frame, nil
}

// ReadFrame reads and returns a frame from the ttyrec. The returned
// frame remains valid only until the next call to ReadFrame.
func (t *TReader) ReadFrame() (*Frame, error) {
	header, err := t.readHeader()
	if err != nil {
		return nil, err
	}
	return t.readFrameBody(header)
}
