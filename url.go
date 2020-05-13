package rbxfetch

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anaminus/iofl"
)

// FilterURL is an iofl.Filter that fetches from a URL.
type FilterURL struct {
	URL           string
	GUID          string
	Client        *http.Client
	CacheMode     CacheMode
	CacheLocation string

	r   io.ReadCloser
	err error
}

// NewFilterURL is an iofl.NewFilter that returns a FilterURL.
func NewFilterURL(params iofl.Params, r io.ReadCloser) (f iofl.Filter, err error) {
	return &FilterURL{r: r,
		URL: params.GetString("URL"),
	}, nil
}

func (f *FilterURL) SetGUID(guid string) {
	f.GUID = guid
}

func (f *FilterURL) SetClient(client *http.Client) {
	f.Client = client
}

func (f *FilterURL) SetCache(mode CacheMode, loc string) {
	f.CacheMode = mode
	f.CacheLocation = loc
}

func (f *FilterURL) Source() io.ReadCloser {
	return f.r
}

func (f *FilterURL) Close() error {
	if f.err != nil {
		return f.err
	}
	if f.err = f.r.Close(); f.err == nil {
		f.err = iofl.Closed
		return nil
	}
	return f.err
}

type statusError struct {
	status int
	msg    string
}

func (s statusError) Error() string {
	return s.msg + " (" + strconv.Itoa(s.status) + ")"
}

func (s statusError) Status() int {
	return s.status
}

func hasStatusError(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := statusError{status: resp.StatusCode, msg: resp.Status}
		return fmt.Errorf("download from %s: %w", resp.Request.URL.String(), err)
	}
	return nil
}

func (f *FilterURL) download(url string) (rc io.ReadCloser, err error) {
	c := f.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	if err := hasStatusError(resp); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

const cacheDirName = "roblox-fetch"

func expandGUID(s, guid string) string {
	return os.Expand(s, func(v string) string {
		switch strings.ToLower(v) {
		case "guid":
			return guid
		}
		return ""
	})
}

func (f *FilterURL) fetch() (rc io.ReadCloser, err error) {
	u := expandGUID(f.URL, f.GUID)
	loc, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	var cacheDir string
	var cachedFilePath string
	var downloaded bool

	switch f.CacheMode {
	case CacheTemp:
		cacheDir = filepath.Join(os.TempDir(), cacheDirName)
	case CachePerm:
		dir, err := os.UserCacheDir()
		if err != nil {
			dir = os.TempDir()
		}
		cacheDir = filepath.Join(dir, cacheDirName)
	case CacheCustom:
		cacheDir = f.CacheLocation
	default:
		goto direct
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	cachedFilePath = filepath.Join(cacheDir, url.PathEscape(loc.Host+loc.Path))

tryCache:
	if cachedFile, err := os.Open(cachedFilePath); err == nil {
		return cachedFile, nil
	}

	if !downloaded {
		if tempFile, err := ioutil.TempFile(cacheDir, "temp"); err == nil {
			tempName := tempFile.Name()

			// Download response body.
			rc, err := f.download(u)
			if err != nil {
				tempFile.Close()
				os.Remove(tempFile.Name())
				return nil, err
			}

			// Write to temp file.
			_, err = io.Copy(tempFile, rc)
			rc.Close()
			if err != nil {
				tempFile.Close()
				os.Remove(tempFile.Name())
				return nil, err
			}

			// Sync temp file.
			err = tempFile.Sync()
			tempFile.Close()
			if err != nil {
				os.Remove(tempFile.Name())
				return nil, err
			}
			downloaded = true

			// Attempt to relocate temp file to cache file.
			if err := os.Rename(tempName, cachedFilePath); err != nil {
				// Rename failed. Data is still in temp file, so we'll reuse that.
				cachedFilePath = tempName
			}
			goto tryCache
		}
	}

direct:
	// Return response body directly.
	return f.download(u)
}

func (f *FilterURL) Read(p []byte) (n int, err error) {
	if f.err != nil {
		return 0, f.err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if f.r == nil {
		f.r, err = f.fetch()
		if err != nil {
			f.err = err
			return 0, err
		}
	}
	return f.r.Read(p)
}
