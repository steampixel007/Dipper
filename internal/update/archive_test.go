package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarFixture(t *testing.T, entries ...tar.Header) []byte {
	t.Helper()
	var body bytes.Buffer
	compressed := gzip.NewWriter(&body)
	writer := tar.NewWriter(compressed)
	for _, entry := range entries {
		if entry.Typeflag == tar.TypeReg {
			entry.Size = int64(len(entry.Name))
			entry.Mode = 0755
		}
		if err := writer.WriteHeader(&entry); err != nil {
			t.Fatal(err)
		}
		if entry.Typeflag == tar.TypeReg {
			if _, err := writer.Write([]byte(entry.Name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestUnixUpdateDownloadVerifiesWholePayload(t *testing.T) {
	for _, mode := range []string{ModeMacOS, ModeLinux, ModeDebian} {
		t.Run(mode, func(t *testing.T) {
			entries := []tar.Header{{Name: "dipper", Typeflag: tar.TypeReg}}
			platform, suffix := "linux", ".tar.gz"
			if mode == ModeMacOS {
				platform = "darwin"
				entries = []tar.Header{
					{Name: "Dipper.app/", Typeflag: tar.TypeDir, Mode: 0755},
					{Name: "Dipper.app/Contents/Info.plist", Typeflag: tar.TypeReg},
					{Name: "Dipper.app/Contents/MacOS/Dipper", Typeflag: tar.TypeReg},
					{Name: "Dipper.app/Contents/Resources/icon.icns", Typeflag: tar.TypeReg},
				}
			}
			body := tarFixture(t, entries...)
			if mode == ModeDebian {
				suffix, body = ".deb", []byte("verified package fixture")
			}
			name := "dipper_0.1.3_" + platform + "_amd64" + suffix
			release := fixtureRelease(t)
			var asset Asset
			for _, candidate := range release.Assets {
				if candidate.Name == name {
					asset = candidate
				}
			}
			offer := Offer{Release: release, Package: asset, Checksums: release.Assets[8], Available: true, Mode: mode}
			digest := sha256.Sum256(body)
			sums := hex.EncodeToString(digest[:]) + "  " + name + "\n"
			offer.Package.Size, offer.Checksums.Size = int64(len(body)), int64(len(sums))
			client := fixtureClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
					writer.Write([]byte(sums))
					return
				}
				writer.Write(body)
			})
			downloaded, err := client.Download(context.Background(), offer, t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if downloaded.Mode != mode {
				t.Fatalf("lost installation mode: %+v", downloaded)
			}
			if err := VerifyUpdateDownload(downloaded); err != nil {
				t.Fatalf("verified update cannot be installed: %v", err)
			}
			target := downloaded.Executable
			switch mode {
			case ModeMacOS:
				target = filepath.Join(downloaded.Bundle, "Contents", "Resources", "icon.icns")
			case ModeDebian:
				target = downloaded.Package
			}
			if err := os.WriteFile(target, []byte("changed after download"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := VerifyUpdateDownload(downloaded); err == nil {
				t.Fatal("modified payload was accepted for restart update")
			}
		})
	}
}

func TestTarExtractionRejectsUnsafeEntries(t *testing.T) {
	for _, entry := range []tar.Header{
		{Name: ".", Typeflag: tar.TypeDir},
		{Name: "..", Typeflag: tar.TypeDir},
		{Name: "../dipper", Typeflag: tar.TypeReg},
		{Name: "/dipper", Typeflag: tar.TypeReg},
		{Name: "dipper:stream", Typeflag: tar.TypeReg},
		{Name: "dipper", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		{Name: "dipper", Typeflag: tar.TypeLink, Linkname: "other"},
		{Name: "extra", Typeflag: tar.TypeReg},
		{Name: "dipper/", Typeflag: tar.TypeDir},
	} {
		t.Run(entry.Name, func(t *testing.T) {
			directory := t.TempDir()
			archive := filepath.Join(directory, "update.tar.gz")
			if err := os.WriteFile(archive, tarFixture(t, entry), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := extractTar(context.Background(), archive, directory, ModeLinux); err == nil {
				t.Fatal("unsafe archive entry accepted")
			}
		})
	}
}

func TestBundleDigestRejectsAddedOrMissingResources(t *testing.T) {
	directory := t.TempDir()
	archive := filepath.Join(directory, "update.tar.gz")
	body := tarFixture(t,
		tar.Header{Name: "Dipper.app/Contents/Info.plist", Typeflag: tar.TypeReg},
		tar.Header{Name: "Dipper.app/Contents/MacOS/Dipper", Typeflag: tar.TypeReg},
	)
	if err := os.WriteFile(archive, body, 0600); err != nil {
		t.Fatal(err)
	}
	_, bundle, err := extractTar(context.Background(), archive, directory, ModeMacOS)
	if err != nil {
		t.Fatal(err)
	}
	original, err := BundleDigest(bundle)
	if err != nil {
		t.Fatal(err)
	}
	extra := filepath.Join(bundle, "unexpected")
	if err := os.WriteFile(extra, []byte("unverified additional file"), 0600); err != nil {
		t.Fatal(err)
	}
	added, err := BundleDigest(bundle)
	if err != nil || added == original {
		t.Fatalf("added bundle file was not detected: %v", err)
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(bundle, "Contents", "Info.plist")); err != nil {
		t.Fatal(err)
	}
	missing, err := BundleDigest(bundle)
	if err != nil || missing == original {
		t.Fatalf("missing bundle metadata was not detected: %v", err)
	}
}
