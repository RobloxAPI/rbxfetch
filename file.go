package rbxfetch

import (
	"io"
	"os"

	"github.com/anaminus/iofl"
)

// FilterFile is an iofl.Filter that fetches from a file.
type FilterFile struct {
	Path string
	GUID string

	r   io.ReadCloser
	err error
}

// NewFilterFile is an iofl.NewFilter that returns a FilterFile.
func NewFilterFile(params iofl.Params, r io.ReadCloser) (f iofl.Filter, err error) {
	return &FilterFile{r: r,
		Path: params.GetString("Path"),
	}, nil
}

func (f *FilterFile) SetGUID(guid string) {
	f.GUID = guid
}

func (f *FilterFile) Source() io.ReadCloser {
	return f.r
}

func (f *FilterFile) Close() error {
	if f.err != nil {
		return f.err
	}
	if f.err = f.r.Close(); f.err == nil {
		f.err = iofl.Closed
		return nil
	}
	return f.err
}

func (f *FilterFile) Read(p []byte) (n int, err error) {
	if f.err != nil {
		return 0, f.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if f.r == nil {
		if f.r, err = os.Open(expandGUID(f.Path, f.GUID)); err != nil {
			f.err = err
			return 0, err
		}
	}
	return f.r.Read(p)
}
