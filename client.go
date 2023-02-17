// The rbxfetch package retrieves information about Roblox builds.
package rbxfetch

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/anaminus/iofl"
	"github.com/robloxapi/rbxdump/histlog"
)

// CacheMode specifies how data is cached between calls.
type CacheMode int

const (
	// Data is never cached.
	CacheNone CacheMode = iota
	// Data is cached in the temporary directory.
	CacheTemp
	// Data is cached in the user cache directory. If unavailable, the
	// temporary directory is used instead.
	CachePerm
	// Data is cached to a custom directory specified by CacheLocation.
	CacheCustom
)

// Version represents a Roblox version number.
type Version = histlog.Version

// Build represents information about a single Roblox build.
type Build struct {
	Type    string
	GUID    string
	Date    time.Time
	Version Version
}

func (b *Build) UnmarshalJSON(p []byte) (err error) {
	var s string
	if err = json.Unmarshal(p, &s); err == nil {
		b.GUID = s
		return nil
	}
	type jBuild Build
	var build jBuild
	if err = json.Unmarshal(p, &build); err == nil {
		*b = Build(build)
		return nil
	}
	return err
}

// Client is used to perform the fetching of information. It controls where
// data is retrieved from, and how the data is cached.
type Client struct {
	// CacheMode specifies how to cache files.
	CacheMode CacheMode
	// CacheLocation specifies the path to store cached files, when CacheMode
	// is CacheCustom.
	CacheLocation string
	// Client is the HTTP client that performs requests.
	Client *http.Client

	methods  map[string][]string
	chainSet *iofl.ChainSet
}

// NewClient returns a client with a default configuration and temporary
// caching. The Client is initialized with the following filters:
//
//     - url: FilterURL
//     - file: FilterFile
//     - zip: FilterZip
//     - iconscan: FilterIconScan
//
// Using these filters, the following chains are specified:
//
//     - Latest: Fetches the GUID of the latest build.
//     - Live: Fetches the GUID of the latest live 32-bit Studio build.
//     - Live64: Fetches the GUID of the latest live 64-bit Studio build.
//     - Builds: Fetches a list of builds.
//     - APIDump: Fetches the API dump of a given GUID.
//     - ReflectionMetadata: Fetches the reflection metadata of a given GUID.
//     - ClassImages: Fetches the class icons of a given GUID.
//     - ExplorerIcons: Fetches the class icons of a given GUID, scanned from
//       the Studio executable.
//
// Finally, the following methods are specified:
//
//     - Builds: Builds
//     - Latest: Latest
//     - APIDump: APIDump
//     - ReflectionMetadata: ReflectionMetadata
//     - ClassImages: ClassImages, ExplorerIcons
//     - Live: Live64, Live
func NewClient() *Client {
	return &Client{
		CacheMode: CacheTemp,
		chainSet:  newDefaultChainSet(),
		methods:   newDefaultMethods(),
	}
}

// Config is used to configure a Client.
type Config struct {
	// Methods specifies the list of chains to be used consecutively for each
	// client method. The result of each chain in the list may be used, or the
	// result of the first chain that doesn't error.
	Methods map[string][]string
	iofl.Config
}

// Config returns a copy of the configuration used by the client.
func (client *Client) Config() Config {
	var config Config

	config.Methods = make(map[string][]string, len(client.methods))
	for name, method := range client.methods {
		m := make([]string, len(method))
		copy(m, method)
		config.Methods[name] = m
	}

	config.Config = client.chainSet.Config()

	return config
}

// SetConfig uses config to configure the client.
func (client *Client) SetConfig(config Config) error {
	client.methods = make(map[string][]string, len(config.Methods))
	for name, method := range config.Methods {
		m := make([]string, len(method))
		copy(m, method)
		client.methods[name] = m
	}

	return client.chainSet.SetConfig(config.Config)
}

// applyGUID applies guid to the chain of filters.
func applyGUID(filter iofl.Filter, guid string) {
	type guider interface {
		iofl.Filter
		SetGUID(guid string)
	}
	iofl.Apply(filter, func(f io.ReadCloser) error {
		if f, ok := f.(guider); ok {
			f.SetGUID(guid)
		}
		return nil
	})
}

// applyClient applies client and cache to the chain of filters.
func applyClient(filter iofl.Filter, client *http.Client, cacheMode CacheMode, cacheLoc string) {
	type clienter interface {
		iofl.Filter
		SetClient(client *http.Client)
		SetCache(mode CacheMode, loc string)
	}
	iofl.Apply(filter, func(f io.ReadCloser) error {
		if f, ok := f.(clienter); ok {
			f.SetClient(client)
			f.SetCache(cacheMode, cacheLoc)
		}
		return nil
	})
}

// resolve resolves the given chain using the given GUID. If guid is empty, then
// the chain is assumed to be a build endpoint, and will not be cached.
func (client *Client) resolve(chain string, guid string) (filter iofl.Filter, err error) {
	f, err := client.chainSet.Resolve(chain, nil)
	if err != nil {
		return nil, err
	}
	if guid == "" {
		// Disable caching of build endpoints.
		applyClient(f, client.Client, CacheNone, "")
	} else {
		applyClient(f, client.Client, client.CacheMode, client.CacheLocation)
		applyGUID(f, guid)
	}
	return f, nil
}

// Latest returns the GUID of the latest build, which can be passed to other
// methods to fetch data corresponding to the latest version. Latest uses the
// result of the first chain that does not error. Returns an empty string if no
// "Latest" method is configured.
//
// The content of a chain is expected to be a raw GUID.
func (client *Client) Latest() (guid string, err error) {
	for _, chain := range client.methods["Latest"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, ""); err != nil {
			continue
		}
		var b []byte
		b, err = ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		return string(b), nil
	}
	return guid, err
}

// Live returns the GUIDs of the current live builds, which can be passed to
// other methods to fetch data corresponding to current live versions. Live
// visits every configured chain, returning a list of GUIDs, or the first error
// that occurs. Returns an empty slice if no "Live" method is configured.
//
// The content of a chain is expected to be a JSON string containing the GUID.
func (client *Client) Live() (guids []string, err error) {
	for _, chain := range client.methods["Live"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, ""); err != nil {
			return nil, err
		}
		var guid string
		err = json.NewDecoder(f).Decode(&guid)
		f.Close()
		if err != nil {
			return nil, err
		}
		guids = append(guids, guid)
	}
	return guids, nil
}

// Builds returns a list of available builds. Returns nil if no "Builds" method
// is configured.
//
// The content of a chain is expected to be a histlog stream.
func (client *Client) Builds() (builds []Build, err error) {
	for _, chain := range client.methods["Builds"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, ""); err != nil {
			continue
		}
		var b []byte
		b, err = ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			continue
		}
		stream := histlog.Lex(b)
		for i := 0; i < len(stream); i++ {
			switch job := stream[i].(type) {
			case *histlog.Job:
				builds = append(builds, Build{
					Type:    job.Build,
					GUID:    job.GUID,
					Date:    job.Time,
					Version: job.Version,
				})
			}
		}
		return builds, nil
	}
	return nil, err
}

// APIDump returns the API dump of the given GUID. Returns nil if no "APIDump"
// method is configured.
func (client *Client) APIDump(guid string) (rc io.ReadCloser, err error) {
	for _, chain := range client.methods["APIDump"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, guid); err != nil {
			continue
		}
		return f, nil
	}
	return nil, err
}

// ReflectionMetadata returns the reflection metadata for the given GUID.
// Returns nil if no "ReflectionMetadata" method is configured.
func (client *Client) ReflectionMetadata(guid string) (rc io.ReadCloser, err error) {
	for _, chain := range client.methods["ReflectionMetadata"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, guid); err != nil {
			continue
		}
		return f, nil
	}
	return nil, err
}

// ClassImages returns the class explorer icons for the given GUID. Returns nil
// if no "ClassImages" method is configured.
func (client *Client) ClassImages(guid string) (rc io.ReadCloser, err error) {
	for _, chain := range client.methods["ClassImages"] {
		var f iofl.Filter
		if f, err = client.resolve(chain, guid); err != nil {
			continue
		}
		return f, nil
	}
	return nil, err
}

// Method runs the configured method for the given GUID. Returns nil if no such
// method is configured.
func (client *Client) Method(method, guid string) (rc io.ReadCloser, err error) {
	for _, chain := range client.methods[method] {
		var f iofl.Filter
		if f, err = client.resolve(chain, guid); err != nil {
			continue
		}
		return f, nil
	}
	return nil, err
}
