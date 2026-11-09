package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/steampixel007/Dipper/internal/proxy"
)

const repository = "steampixel007/Dipper"

const (
	ModeWindows = "windows"
	ModeMacOS   = "macos"
	ModeLinux   = "linux"
	ModeDebian  = "debian"
)

var versionPattern = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?(\+([0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*))?$`)

func parseVersion(value string) ([]string, []string, error) {
	match := versionPattern.FindStringSubmatch(value)
	if match == nil {
		return nil, nil, fmt.Errorf("invalid semantic version %q", value)
	}
	var pre []string
	if match[5] != "" {
		pre = strings.Split(match[5], ".")
		for _, part := range pre {
			if numeric(part) && len(part) > 1 && part[0] == '0' {
				return nil, nil, fmt.Errorf("invalid prerelease %q", value)
			}
		}
	}
	return match[1:4], pre, nil
}

func numeric(value string) bool {
	return strings.Trim(value, "0123456789") == ""
}

func compareNumber(left, right string) int {
	if len(left) != len(right) {
		if len(left) < len(right) {
			return -1
		}
		return 1
	}
	return strings.Compare(left, right)
}

func Compare(left, right string) (int, error) {
	leftCore, leftPre, err := parseVersion(left)
	if err != nil {
		return 0, err
	}
	rightCore, rightPre, err := parseVersion(right)
	if err != nil {
		return 0, err
	}
	for index := range leftCore {
		if result := compareNumber(leftCore[index], rightCore[index]); result != 0 {
			return result, nil
		}
	}
	if len(leftPre) == 0 || len(rightPre) == 0 {
		switch {
		case len(leftPre) > 0:
			return -1, nil
		case len(rightPre) > 0:
			return 1, nil
		default:
			return 0, nil
		}
	}
	for index := 0; index < len(leftPre) && index < len(rightPre); index++ {
		leftPart, rightPart := leftPre[index], rightPre[index]
		if leftPart == rightPart {
			continue
		}
		if numeric(leftPart) && numeric(rightPart) {
			return compareNumber(leftPart, rightPart), nil
		}
		if numeric(leftPart) != numeric(rightPart) {
			if numeric(leftPart) {
				return -1, nil
			}
			return 1, nil
		}
		return strings.Compare(leftPart, rightPart), nil
	}
	return compareNumber(fmt.Sprint(len(leftPre)), fmt.Sprint(len(rightPre))), nil
}

func Development(version string) bool {
	return version == "dev" || version == "(devel)" || version == ""
}

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Release struct {
	Tag        string  `json:"tag_name"`
	Notes      string  `json:"body"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Offer struct {
	Release     Release
	Package     Asset
	Checksums   Asset
	Available   bool
	Development bool
	Portable    bool
	Mode        string
}

type Client struct {
	http       *http.Client
	api        string
	assetValid func(string, string, string) bool
}

func NewClient() *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxy.Func
	return &Client{http: &http.Client{Transport: transport, Timeout: 30 * time.Minute, CheckRedirect: officialRedirect}, api: "https://api.github.com/repos/" + repository + "/releases", assetValid: officialAsset}
}

func officialRedirect(request *http.Request, via []*http.Request) error {
	if len(via) >= 10 || request.URL.Scheme != "https" || request.URL.User != nil {
		return errors.New("unsafe release redirect")
	}
	switch request.URL.Host {
	case "github.com", "api.github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com", "github-releases.githubusercontent.com":
		return nil
	default:
		return errors.New("release redirect is not an official GitHub host")
	}
}

func WindowsInstallationOffer(offer Offer, executable string) (Offer, error) {
	return installationOffer(offer, executable, "windows", nil)
}

func SelfUpdating(offer Offer) bool {
	switch offer.Mode {
	case ModeWindows, ModeMacOS, ModeLinux, ModeDebian:
		return true
	}
	return offer.Portable && offer.Mode == ""
}

func InstallationOffer(ctx context.Context, offer Offer, executable, platform string) (Offer, error) {
	return installationOffer(offer, executable, platform, func(target string) (bool, error) {
		return DebianInstallation(ctx, target)
	})
}

func installationOffer(offer Offer, executable, platform string, debian func(string) (bool, error)) (Offer, error) {
	if !offer.Available {
		return offer, nil
	}
	var suffix, mode string
	switch platform {
	case "windows":
		suffix, mode = ".zip", ModeWindows
	case "darwin":
		suffix, mode = ".tar.gz", ModeMacOS
	case "linux":
		suffix, mode = ".tar.gz", ModeLinux
		if debian != nil {
			installed, err := debian(executable)
			if err != nil {
				return Offer{}, fmt.Errorf("identify installed package: %w", err)
			}
			if installed {
				suffix, mode = ".deb", ModeDebian
			}
		}
	default:
		return Offer{}, errors.New("unsupported update platform")
	}
	stem := offer.Package.Name
	for _, extension := range []string{"_setup.exe", ".tar.gz", ".zip", ".dmg", ".deb"} {
		if strings.HasSuffix(stem, extension) {
			stem = strings.TrimSuffix(stem, extension)
			break
		}
	}
	name := stem + suffix
	var selected Asset
	for _, asset := range offer.Release.Assets {
		if asset.Name == name {
			if selected.Name != "" || asset.Size <= 0 || !officialAsset(asset.URL, offer.Release.Tag, name) {
				return Offer{}, errors.New("release contains invalid or duplicate update assets")
			}
			selected = asset
		}
	}
	if selected.Name == "" {
		return Offer{}, errors.New("release does not provide the required verified update package")
	}
	offer.Package, offer.Mode, offer.Portable = selected, mode, mode == ModeWindows
	return offer, nil
}

func DebianInstallation(ctx context.Context, executable string) (bool, error) {
	resolved, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return false, err
	}
	if resolved != "/opt/dipper/dipper" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/usr/bin/dpkg-query", "--search", resolved)
	body, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return false, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read Debian package ownership: %w", err)
	}
	if strings.TrimSpace(string(body)) != "dipper: "+resolved {
		return false, errors.New("unexpected owner of the installed Dipper executable")
	}
	return true, nil
}

func officialAsset(raw, tag, name string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host == "github.com" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" &&
		parsed.Path == "/"+repository+"/releases/download/"+tag+"/"+name
}

func (c *Client) get(ctx context.Context, address string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Dipper-software-update")
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("GitHub HTTP %d (check connection, proxy or rate limit)", response.StatusCode)
	}
	return response, nil
}

func (c *Client) Check(ctx context.Context, current, platform, architecture string) (Offer, error) {
	if !Development(current) {
		if _, _, err := parseVersion(current); err != nil {
			return Offer{}, err
		}
	}
	var newest Release
	for page := 1; page <= 5; page++ {
		response, err := c.get(ctx, fmt.Sprintf("%s?per_page=100&page=%d", c.api, page))
		if err != nil {
			return Offer{}, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, (4<<20)+1))
		response.Body.Close()
		if readErr != nil {
			return Offer{}, readErr
		}
		if len(body) > 4<<20 {
			return Offer{}, errors.New("release response too large")
		}
		var releases []Release
		if err := json.Unmarshal(body, &releases); err != nil {
			return Offer{}, fmt.Errorf("decode releases: %w", err)
		}
		for _, release := range releases {
			_, pre, err := parseVersion(release.Tag)
			if release.Draft || release.Prerelease || err != nil || len(pre) > 0 {
				continue
			}
			comparison, _ := Compare(release.Tag, newest.Tag)
			if newest.Tag == "" || comparison > 0 {
				newest = release
			}
		}
		if !strings.Contains(response.Header.Get("Link"), `rel="next"`) {
			break
		}
		if page == 5 {
			return Offer{}, errors.New("release pagination exceeds safety limit")
		}
	}
	if newest.Tag == "" {
		return Offer{}, errors.New("no stable release was returned")
	}
	offer := Offer{Release: newest, Development: Development(current)}
	comparison, _ := Compare(newest.Tag, current)
	offer.Available = offer.Development || comparison > 0
	if !offer.Available {
		return offer, nil
	}
	stem := "dipper_" + strings.TrimPrefix(newest.Tag, "v") + "_" + platform + "_" + architecture
	names := []string{}
	switch platform {
	case "windows":
		names = []string{stem + ".zip", stem + "_setup.exe"}
	case "darwin":
		names = []string{stem + ".tar.gz", stem + ".dmg"}
	case "linux":
		names = []string{stem + ".tar.gz", stem + ".deb"}
	}
	find := func(name string) (Asset, error) {
		var found Asset
		for _, asset := range newest.Assets {
			if asset.Name != name {
				continue
			}
			if found.Name != "" || asset.Size <= 0 || !c.assetValid(asset.URL, newest.Tag, asset.Name) {
				return Asset{}, errors.New("invalid or duplicate release asset")
			}
			found = asset
		}
		return found, nil
	}
	for _, name := range names {
		asset, err := find(name)
		if err != nil {
			return offer, err
		}
		if asset.Name != "" {
			offer.Package = asset
			break
		}
	}
	var err error
	offer.Checksums, err = find("SHA256SUMS")
	if err != nil {
		return offer, err
	}
	if offer.Package.Name == "" || offer.Checksums.Name == "" {
		return offer, fmt.Errorf("release has no verified package for %s/%s", platform, architecture)
	}
	offer.Portable = platform == "windows" && strings.HasSuffix(offer.Package.Name, ".zip")
	switch {
	case offer.Portable:
		offer.Mode = ModeWindows
	case platform == "darwin" && strings.HasSuffix(offer.Package.Name, ".tar.gz"):
		offer.Mode = ModeMacOS
	case platform == "linux" && strings.HasSuffix(offer.Package.Name, ".tar.gz"):
		offer.Mode = ModeLinux
	case platform == "linux" && strings.HasSuffix(offer.Package.Name, ".deb"):
		offer.Mode = ModeDebian
	}
	return offer, nil
}
