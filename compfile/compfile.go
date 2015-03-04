// Package compfile returns ReadClosers for files that are possibly
// compressed, guessing the compression type based on the file
// extension.
package compfile

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)

type IoWrapper func(io.ReadCloser) (io.ReadCloser, error)

type readWrapper struct {
	orig io.ReadCloser
	io.Reader
}

func (b *readWrapper) Close() error {
	return b.orig.Close()
}

func bzipWrapper(r io.ReadCloser) (io.ReadCloser, error) {
	return &readWrapper{orig: r, Reader: bzip2.NewReader(r)}, nil
}

type readCloseWrapper struct {
	orig io.ReadCloser
	io.ReadCloser
}

func (g *readCloseWrapper) Close() error {
	g.ReadCloser.Close()
	return g.orig.Close()
}

func gzipWrapper(r io.ReadCloser) (io.ReadCloser, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		r.Close()
		return nil, err
	}
	return &readCloseWrapper{orig: r, ReadCloser: gz}, nil
}

var CompressionType = map[string]IoWrapper{
	".bz2":   bzipWrapper,
	".bzip2": bzipWrapper,
	".gz":    gzipWrapper,
}

const DefaultBufferSize = 65536

// Open opens a file (possibly compressed) and returns a suitable
// buffered ReadCloser.
func Open(file string) (io.ReadCloser, error) {
	br, err := OpenBufferedSize(file, DefaultBufferSize)
	if err != nil {
		return nil, err
	}

	if decompressor, ok := CompressionType[filepath.Ext(file)]; ok {
		return decompressor(br)
	}
	return br, nil
}

// OpenBufferedSize opens a file for buffered I/O with the given
// buffer size.
func OpenBufferedSize(file string, size int) (io.ReadCloser, error) {
	inf, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	return &readWrapper{orig: inf, Reader: bufio.NewReaderSize(inf, size)}, nil
}
