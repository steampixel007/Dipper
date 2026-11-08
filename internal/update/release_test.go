package update

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/steampixel007/Dipper/internal/proxy"
)

func fixtureRelease(t *testing.T) Release {
	t.Helper()
	body, err := os.ReadFile("testdata/release.json")
	if err != nil {
		t.Fatal(err)
	}
	var release Release
	if err := json.Unmarshal(body, &release); err != nil {
		t.Fatal(err)
	}
	return release
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func fixtureClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	base, _ := url.Parse(server.URL)
	client := NewClient()
	client.http.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "api.github.com" && request.URL.Host != "github.com" {
			return nil, errors.New("test blocked unexpected network host")
		}
		copy := request.Clone(request.Context())
		address := *request.URL
		address.Scheme, address.Host = base.Scheme, base.Host
		copy.URL = &address
		return server.Client().Transport.RoundTrip(copy)
	})
	return client
}

func TestSemVerOrder(t *testing.T) {
	for _, pair := range [][2]string{
		{"1.0.0-alpha", "1.0.0-alpha.1"}, {"1.0.0-alpha.1", "1.0.0-alpha.beta"},
		{"1.0.0-beta.2", "1.0.0-beta.11"}, {"1.0.0-rc.1", "1.0.0"},
		{"1.9.999", "1.10.0"}, {"99999999999999999999.0.0", "100000000000000000000.0.0"},
	} {
		if result, err := Compare(pair[0], pair[1]); err != nil || result >= 0 {
			t.Errorf("%q must precede %q: %d %v", pair[0], pair[1], result, err)
		}
		if result, err := Compare(pair[1], pair[0]); err != nil || result <= 0 {
			t.Errorf("reverse comparison: %d %v", result, err)
		}
	}
	if result, err := Compare("v1.2.3+build.20", "1.2.3+other"); result != 0 || err != nil {
		t.Fatalf("build metadata affects order: %d %v", result, err)
	}
	for _, value := range []string{"01.0.0", "1.0", "1.0.0-01", "v1.0.0-", "dev-dirty"} {
		if _, err := Compare(value, "1.0.0"); err == nil {
			t.Errorf("accepted invalid version %q", value)
		}
	}
}

func TestReleaseSelection(t *testing.T) {
	release := fixtureRelease(t)
	beta := release
	beta.Tag, beta.Prerelease = "v9.0.0-beta.1", true
	draft := release
	draft.Tag, draft.Draft = "v8.0.0", true
	client := fixtureClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("User-Agent") != "Dipper-software-update" {
			t.Error("missing update user agent")
		}
		json.NewEncoder(writer).Encode([]Release{beta, release, draft})
	})
	for _, target := range []struct{ platform, arch, suffix string }{
		{"windows", "amd64", ".zip"}, {"darwin", "arm64", ".dmg"}, {"darwin", "amd64", ".dmg"}, {"linux", "amd64", ".deb"},
	} {
		offer, err := client.Check(context.Background(), "v0.1.2", target.platform, target.arch)
		if err != nil || !offer.Available || offer.Release.Tag != "v0.1.3" || !strings.HasSuffix(offer.Package.Name, target.suffix) || offer.Checksums.Name != "SHA256SUMS" {
			t.Fatalf("%+v: %+v %v", target, offer, err)
		}
	}
	for _, current := range []string{"0.1.3", "0.2.0", "0.1.3+local"} {
		offer, err := client.Check(context.Background(), current, "windows", "amd64")
		if err != nil || offer.Available {
			t.Fatalf("offered downgrade/equal version for %s: %+v %v", current, offer, err)
		}
	}
	for _, current := range []string{"dev", "", "(devel)", "0.1.3-beta.1"} {
		offer, err := client.Check(context.Background(), current, "windows", "amd64")
		if err != nil || !offer.Available || offer.Development != Development(current) {
			t.Fatalf("development/prerelease comparison %s: %+v %v", current, offer, err)
		}
	}
	if _, err := client.Check(context.Background(), "0.1.2", "linux", "arm64"); err == nil {
		t.Fatal("invented an unavailable Linux ARM64 asset")
	}
}

func TestReleaseFailuresAndPagination(t *testing.T) {
	for _, scenario := range []string{"http", "malformed", "empty", "missing", "duplicate", "unofficial", "pagination"} {
		t.Run(scenario, func(t *testing.T) {
			release := fixtureRelease(t)
			client := fixtureClient(t, func(writer http.ResponseWriter, request *http.Request) {
				switch scenario {
				case "http":
					writer.WriteHeader(403)
					return
				case "malformed":
					writer.Write([]byte("{"))
					return
				case "empty":
					writer.Write([]byte("[]"))
					return
				case "missing":
					release.Assets = release.Assets[:1]
				case "duplicate":
					release.Assets = append(release.Assets, release.Assets[6])
				case "unofficial":
					release.Assets[6].URL = "https://example.com/malware.zip"
				case "pagination":
					writer.Header().Set("Link", `<https://example.com/ignored>; rel="next"`)
				}
				json.NewEncoder(writer).Encode([]Release{release})
			})
			if _, err := client.Check(context.Background(), "0.1.2", "windows", "amd64"); err == nil {
				t.Fatal("failed release lookup was reported as success")
			}
		})
	}
	release := fixtureRelease(t)
	client := fixtureClient(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("page") == "1" {
			writer.Header().Set("Link", `<https://api.github.com/next>; rel="next"`)
			writer.Write([]byte("[]"))
			return
		}
		json.NewEncoder(writer).Encode([]Release{release})
	})
	if offer, err := client.Check(context.Background(), "0.1.2", "windows", "amd64"); err != nil || !offer.Available {
		t.Fatalf("did not follow official pagination: %+v %v", offer, err)
	}
}

func TestUpdateProxyAndRedirects(t *testing.T) {
	proxy.Configure(proxy.ModeCustom, "http://127.0.0.1:12345")
	t.Cleanup(func() { proxy.Configure(proxy.ModeAuto, "") })
	client := NewClient()
	request, _ := http.NewRequest(http.MethodGet, client.api, nil)
	address, err := client.http.Transport.(*http.Transport).Proxy(request)
	if err != nil || address.String() != "http://127.0.0.1:12345" {
		t.Fatalf("update ignored configured proxy: %v %v", address, err)
	}
	for _, raw := range []string{"http://github.com/file", "https://example.com/file", "https://github.com.evil.test/file", "https://user@github.com/file"} {
		request, _ := http.NewRequest(http.MethodGet, raw, nil)
		if officialRedirect(request, nil) == nil {
			t.Errorf("accepted unsafe redirect %s", raw)
		}
	}
	request, _ = http.NewRequest(http.MethodGet, "https://release-assets.githubusercontent.com/file?token=ephemeral", nil)
	if err := officialRedirect(request, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsInstalledCopyUsesSetup(t *testing.T) {
	offer := Offer{Release: fixtureRelease(t), Package: fixtureRelease(t).Assets[6], Available: true, Portable: true}
	dir := t.TempDir()
	path := filepath.Join(dir, "Dipper.exe")
	portable, err := WindowsInstallationOffer(offer, path)
	if err != nil || !portable.Portable {
		t.Fatal("ZIP copy should use portable replacement")
	}
	if err := os.WriteFile(filepath.Join(dir, "Uninstall.exe"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := WindowsInstallationOffer(offer, path)
	if err != nil || installed.Portable || !strings.HasSuffix(installed.Package.Name, "_setup.exe") {
		t.Fatalf("installed copy must use setup: %+v %v", installed, err)
	}
	if selected, err := WindowsInstallationOffer(installed, path); err != nil || selected.Package != installed.Package {
		t.Fatalf("setup fallback was rejected: %+v %v", selected, err)
	}
	offer.Release.Assets = append(offer.Release.Assets, installed.Package)
	if _, err := WindowsInstallationOffer(offer, path); err == nil {
		t.Fatal("accepted duplicate setup asset")
	}
	offer.Release.Assets = nil
	if _, err := WindowsInstallationOffer(offer, path); err == nil {
		t.Fatal("invented missing setup asset")
	}
}
