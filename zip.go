package rbxfetch

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/anaminus/iofl"
)

type readAtSeeker interface {
	io.Reader
	io.ReaderAt
	io.Seeker
}

type readAtSeekCloser interface {
	readAtSeeker
	io.Closer
}

type nopCloser struct {
	readAtSeeker
}

func (nopCloser) Close() error { return nil }

type wrapZipCloser struct {
	zc io.Closer
	zf io.ReadCloser
}

func (r *wrapZipCloser) Close() error {
	err0 := r.zf.Close()
	err1 := r.zc.Close()
	if err0 != nil {
		return err0
	}
	if err1 != nil {
		return err1
	}
	return nil
}

func (r *wrapZipCloser) Read(p []byte) (n int, err error) {
	return r.zf.Read(p)
}

// FilterZip is an iofl.Filter that reads a file within a zip source.
type FilterZip struct {
	File string

	r   io.ReadCloser
	zr  io.ReadCloser
	err error
}

// NewFilterZip is an iofl.NewFilter that returns a FilterZip.
func NewFilterZip(params iofl.Params, r io.ReadCloser) (f iofl.Filter, err error) {
	return &FilterZip{r: r,
		File: params.GetString("File"),
	}, nil
}

func (f *FilterZip) Source() io.ReadCloser {
	return f.r
}

func (f *FilterZip) Close() error {
	if f.err != nil {
		return f.err
	}
	if f.zr != nil {
		// zr also closes r.
		if f.err = f.zr.Close(); f.err == nil {
			f.err = iofl.Closed
			return nil
		}
		return f.err
	}
	if f.err = f.r.Close(); f.err == nil {
		f.err = iofl.Closed
		return nil
	}
	return f.err
}

func unzip(r readAtSeekCloser, filename string) (rc io.ReadCloser, err error) {
	// Find size.
	var size int64
	if size, err = r.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	if _, err = r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	// Read zipped files.
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}

	// Find zipped file.
	var zfile *zip.File
	for _, zf := range zr.File {
		if zf.Name != filename {
			continue
		}
		zfile = zf
		break
	}
	if zfile == nil {
		return nil, fmt.Errorf("%q not in archive", filename)
	}
	zf, err := zfile.Open()
	if err != nil {
		return nil, err
	}

	return &wrapZipCloser{zc: r, zf: zf}, nil
}

func (f *FilterZip) Read(p []byte) (n int, err error) {
	if f.err != nil {
		return 0, f.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if f.zr == nil {
		var rc readAtSeekCloser
		switch r := f.r.(type) {
		case readAtSeekCloser:
			rc = r
		default:
			b, err := ioutil.ReadAll(f.r)
			f.r.Close()
			if err != nil {
				f.err = err
				return 0, err
			}
			rc = nopCloser{bytes.NewReader(b)}
		}
		if f.zr, err = unzip(rc, f.File); err != nil {
			f.err = err
			f.r.Close()
			return 0, err
		}
	}
	return f.zr.Read(p)
}
