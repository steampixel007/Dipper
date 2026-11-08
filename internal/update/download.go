package update

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxPackage = 512 << 20

type Download struct {
	Directory, Package, Executable, SHA256, PackageSHA256 string
}

func VerifyDownload(downloaded Download) error {
	return hashMatches(downloaded.Package, downloaded.PackageSHA256)
}

func Checksum(body []byte, name string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	var expected string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 67 || line[64] != ' ' || (line[65] != ' ' && line[65] != '*') {
			return "", errors.New("invalid SHA256SUMS line")
		}
		digest, err := hex.DecodeString(line[:64])
		if err != nil || len(digest) != sha256.Size {
			return "", errors.New("invalid SHA256SUMS digest")
		}
		if line[66:] == name {
			if expected != "" {
				return "", errors.New("duplicate SHA256SUMS entry")
			}
			expected = strings.ToLower(line[:64])
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if expected == "" {
		return "", errors.New("package missing from SHA256SUMS")
	}
	return expected, nil
}

func Digest(path string) (string, error) {
	return fileHash(path)
}

func (c *Client) Download(ctx context.Context, offer Offer, directory string, progress func(int64, int64)) (result Download, err error) {
	if !offer.Available || offer.Package.Size <= 0 || offer.Package.Size > maxPackage ||
		offer.Checksums.Size <= 0 || offer.Checksums.Size > 1<<20 ||
		filepath.Base(offer.Package.Name) != offer.Package.Name ||
		!c.assetValid(offer.Package.URL, offer.Release.Tag, offer.Package.Name) ||
		!c.assetValid(offer.Checksums.URL, offer.Release.Tag, "SHA256SUMS") {
		return result, errors.New("invalid update offer")
	}
	stage, err := os.MkdirTemp(directory, "dipper-update-download-")
	if err != nil {
		return result, err
	}
	result.Directory = stage
	defer func() {
		if err != nil {
			os.RemoveAll(stage)
			result = Download{}
		}
	}()
	response, err := c.get(ctx, offer.Checksums.URL)
	if err != nil {
		return result, err
	}
	sums, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	response.Body.Close()
	if err != nil || len(sums) > 1<<20 || int64(len(sums)) != offer.Checksums.Size {
		return result, errors.New("incomplete or oversized SHA256SUMS")
	}
	expected, err := Checksum(sums, offer.Package.Name)
	if err != nil {
		return result, err
	}
	response, err = c.get(ctx, offer.Package.URL)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	result.Package = filepath.Join(stage, offer.Package.Name)
	file, err := os.OpenFile(result.Package, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	hash := sha256.New()
	writer := &progressWriter{writer: io.MultiWriter(file, hash), total: offer.Package.Size, progress: progress}
	copied, copyErr := io.Copy(writer, io.LimitReader(response.Body, offer.Package.Size+1))
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(copyErr, syncErr, closeErr, ctx.Err()); err != nil {
		return result, err
	}
	if copied != offer.Package.Size || hex.EncodeToString(hash.Sum(nil)) != expected {
		return result, errors.New("download size or SHA-256 mismatch; retry the official download")
	}
	result.PackageSHA256 = expected
	if offer.Portable {
		result.Executable = filepath.Join(stage, "Dipper.exe")
		if err := Extract(ctx, result.Package, result.Executable); err != nil {
			return result, err
		}
		result.SHA256, err = Digest(result.Executable)
	}
	return result, err
}

type progressWriter struct {
	writer   io.Writer
	current  int64
	total    int64
	progress func(int64, int64)
}

func (writer *progressWriter) Write(body []byte) (int, error) {
	count, err := writer.writer.Write(body)
	writer.current += int64(count)
	if writer.progress != nil {
		writer.progress(writer.current, writer.total)
	}
	return count, err
}

func Extract(ctx context.Context, archive, destination string) error {
	if !filepath.IsAbs(destination) || filepath.Base(destination) != "Dipper.exe" {
		return errors.New("unsafe extraction destination")
	}
	parent := filepath.Dir(destination)
	resolved, err := resolveUpdatePath(parent)
	if err != nil || !samePath(parent, resolved) {
		return errors.New("unsafe extraction directory")
	}
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) != 1 {
		return errors.New("Windows update ZIP must contain only Dipper.exe")
	}
	entry := reader.File[0]
	if entry.Name != "Dipper.exe" || !entry.Mode().IsRegular() || entry.UncompressedSize64 == 0 || entry.UncompressedSize64 > maxPackage {
		return errors.New("unsafe Windows update ZIP entry")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	input, err := entry.Open()
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return err
	}
	copied, copyErr := io.Copy(output, &contextReader{ctx: ctx, reader: io.LimitReader(input, maxPackage+1)})
	syncErr := output.Sync()
	closeErr := output.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil || uint64(copied) != entry.UncompressedSize64 {
		os.Remove(destination)
		return fmt.Errorf("extract executable: %w", errors.Join(err, errors.New("incomplete executable")))
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader *contextReader) Read(body []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(body)
}
