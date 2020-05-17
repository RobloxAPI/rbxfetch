package rbxfetch

import (
	"bufio"
	"bytes"
	"image"
	"image/png"
	"io"

	"github.com/anaminus/iofl"
)

// FilterIconScan is an iofl.Filter that scans for an icon sheet image from the
// source.
//
// Because the source may contain multiple images, the following heuristic is
// used: the format of the image is PNG, the height of the image is Size, the
// width is a multiple of Size, and is the first widest such image.
type FilterIconScan struct {
	Size int

	r   io.ReadCloser
	buf bytes.Buffer
	err error
}

// NewFilterIconScan is an iofl.NewFilter that returns a FilterIconScan.
func NewFilterIconScan(params iofl.Params, r io.ReadCloser) (f iofl.Filter, err error) {
	return &FilterIconScan{r: r,
		Size: params.GetInt("Size"),
	}, nil
}

func (f *FilterIconScan) Source() io.ReadCloser {
	return f.r
}

func (f *FilterIconScan) Close() error {
	if f.err != nil {
		return f.err
	}
	if f.err = f.r.Close(); f.err == nil {
		f.err = iofl.Closed
		return nil
	}
	return f.err
}

// readBytes scans until the given delimiter is reached.
func readBytes(r *bufio.Reader, sep []byte) error {
	if len(sep) == 0 {
		return nil
	}
	for {
		if b, err := r.Peek(len(sep)); err != nil {
			return err
		} else if bytes.Equal(b, sep) {
			break
		}
		if _, err := r.Discard(1); err != nil {
			return err
		}
	}
	return nil
}

// scan scans f.r for an image, writing the result to f.buf.
func (f *FilterIconScan) scan() (err error) {
	header := []byte("\x89PNG\r\n\x1a\n")
	var largest image.Image
	for br := bufio.NewReader(f.r); ; {
		// Scan for PNG headers.
		if err := readBytes(br, header); err != nil {
			if err == io.EOF && largest != nil {
				break
			}
			f.r.Close()
			return err
		}
		f.buf.Reset()
		img, err := png.Decode(io.TeeReader(br, &f.buf))
		// Select when height equals Size, and width is multiple of Size.
		if err != nil || img.Bounds().Dy() != f.Size || img.Bounds().Dx()%f.Size != 0 {
			continue
		}
		//  Select first widest.
		if largest == nil || img.Bounds().Dx() > largest.Bounds().Dx() {
			largest = img
		}
	}
	return f.r.Close()
}

func (f *FilterIconScan) Read(p []byte) (n int, err error) {
	if f.err != nil {
		return 0, f.err
	}
	if f.buf.Len() == 0 {
		if err := f.scan(); err != nil {
			f.err = err
			return 0, err
		}
	}
	return f.buf.Read(p)
}
