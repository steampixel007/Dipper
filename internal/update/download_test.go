package update

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		entry.Write([]byte("harmless executable fixture"))
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func TestDownloadVerificationAndCancellation(t *testing.T) {
	for _, scenario := range []string{"success", "checksum", "size", "cancel", "archive", "manual"} {
		t.Run(scenario, func(t *testing.T) {
			body := fixtureZip(t, "Dipper.exe")
			if scenario == "archive" {
				body = fixtureZip(t, "../Dipper.exe")
			}
			digest := sha256.Sum256(body)
			offer := Offer{Release: fixtureRelease(t), Package: fixtureRelease(t).Assets[6], Checksums: fixtureRelease(t).Assets[8], Available: true, Portable: scenario != "manual"}
			sums := hex.EncodeToString(digest[:]) + "  " + offer.Package.Name + "\n"
			if scenario == "checksum" {
				sums = strings.Repeat("0", 64) + "  " + offer.Package.Name + "\n"
			}
			offer.Package.Size, offer.Checksums.Size = int64(len(body)), int64(len(sums))
			if scenario == "size" {
				offer.Package.Size++
			}
			client := fixtureClient(t, func(writer http.ResponseWriter, request *http.Request) {
				if strings.HasSuffix(request.URL.Path, "SHA256SUMS") {
					writer.Write([]byte(sums))
				} else {
					writer.Write(body)
				}
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dir := t.TempDir()
			var done, total int64
			downloaded, err := client.Download(ctx, offer, dir, func(current, size int64) {
				done, total = current, size
				if scenario == "cancel" {
					cancel()
				}
			})
			if scenario == "success" || scenario == "manual" {
				if err != nil || done != int64(len(body)) || total != int64(len(body)) {
					t.Fatalf("download/progress: %+v %v %d/%d", downloaded, err, done, total)
				}
				if err := VerifyDownload(downloaded); err != nil {
					t.Fatal(err)
				}
				if scenario == "success" {
					if err := hashMatches(downloaded.Executable, downloaded.SHA256); err != nil {
						t.Fatal(err)
					}
				}
				os.WriteFile(downloaded.Package, []byte("tampered after verification"), 0600)
				if err := VerifyDownload(downloaded); err == nil {
					t.Fatal("tampered installation package accepted")
				}
			} else {
				if err == nil || downloaded.Directory != "" {
					t.Fatalf("unsafe/incomplete download accepted: %+v %v", downloaded, err)
				}
				children, _ := os.ReadDir(dir)
				if len(children) != 0 {
					t.Fatal("failed download left staged files")
				}
				if scenario == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			}
		})
	}
}

func TestChecksumAndArchiveErrors(t *testing.T) {
	valid := strings.Repeat("a", 64) + "  package.zip\n"
	for _, body := range []string{"", "bad", strings.Repeat("z", 64) + "  package.zip", valid + valid, valid[:64] + " xpackage.zip"} {
		if _, err := Checksum([]byte(body), "package.zip"); err == nil {
			t.Errorf("accepted malformed/missing/duplicate checksum %q", body)
		}
	}
	if value, err := Checksum([]byte(strings.Replace(valid, "  ", " *", 1)), "package.zip"); err != nil || value != strings.Repeat("a", 64) {
		t.Fatalf("binary checksum syntax: %s %v", value, err)
	}
	for _, names := range [][]string{{}, {"../Dipper.exe"}, {"bin/Dipper.exe"}, {"/Dipper.exe"}, {"Dipper.exe", "other.exe"}, {"Dipper.exe:stream"}, {"dipper.exe"}} {
		dir := t.TempDir()
		archive := filepath.Join(dir, "update.zip")
		os.WriteFile(archive, fixtureZip(t, names...), 0600)
		if err := Extract(context.Background(), archive, filepath.Join(dir, "Dipper.exe")); err == nil {
			t.Errorf("accepted unsafe archive %+v", names)
		}
	}
	dir := t.TempDir()
	archive, target := filepath.Join(dir, "update.zip"), filepath.Join(dir, "Dipper.exe")
	os.WriteFile(archive, fixtureZip(t, "Dipper.exe"), 0600)
	os.WriteFile(target, []byte("existing executable"), 0600)
	if err := Extract(context.Background(), archive, target); err == nil {
		t.Fatal("overwrote existing extraction target")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	os.Remove(target)
	if err := Extract(ctx, archive, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("extraction did not cancel: %v", err)
	}
	if err := Extract(context.Background(), archive, filepath.Join(dir, "other.exe")); err == nil {
		t.Fatal("accepted unexpected executable destination")
	}
}
