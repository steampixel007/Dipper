package update

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	failureReportName    = "failure.json"
	maxFailureReportSize = 16 << 10
)

var errInvalidFailureReport = errors.New("invalid update failure report")

type updateFailureReport struct {
	Message string `json:"message"`
}

func updateStageRoots(executable string) []string {
	root := filepath.Clean(filepath.Dir(executable))
	if runtime.GOOS == "darwin" {
		if bundle, err := applicationBundle(executable); err == nil {
			root = filepath.Dir(bundle)
		}
	}
	candidates := []string{root}
	if runtime.GOOS == "linux" {
		candidates = append(candidates, filepath.Clean(os.TempDir()))
		if debianRoot, err := debianStageRoot(); err == nil {
			candidates = append(candidates, debianRoot)
		}
	}
	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if !filepath.IsAbs(candidate) {
			continue
		}
		if resolved, err := resolveUpdatePath(candidate); err == nil {
			candidate = resolved
		}
		duplicate := false
		for _, root := range roots {
			if samePath(root, candidate) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			roots = append(roots, candidate)
		}
	}
	return roots
}

func persistUpdateFailure(stage string, cause error) error {
	if cause == nil {
		return errors.New("update failure cause is missing")
	}
	if !filepath.IsAbs(stage) || filepath.Clean(stage) != stage ||
		!strings.HasPrefix(filepath.Base(stage), ".dipper-update-") {
		return errors.New("invalid update failure stage")
	}
	info, err := os.Lstat(stage)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("update failure stage is not a directory")
	}
	resolved, err := resolveUpdatePath(stage)
	if err != nil {
		return err
	}
	if !samePath(resolved, stage) {
		return errors.New("update failure stage is redirected")
	}
	body, err := json.Marshal(updateFailureReport{Message: helperFailureText + cause.Error()})
	if err != nil {
		return err
	}
	body = append(body, '\n')
	path := filepath.Join(stage, failureReportName)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(body)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// ConsumeFailureReport returns the newest update failure recorded beside an
// installed application or in a package-update staging directory. The failed
// stage is retained for diagnosis; only the report marker is consumed.
func ConsumeFailureReport(executable string) (string, error) {
	var message string
	var newest time.Time
	var problems []error
	for _, root := range updateStageRoots(executable) {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || !strings.HasPrefix(name, ".dipper-update-") {
				continue
			}
			stage := filepath.Join(root, name)
			if !validFailureStage(root, stage) {
				continue
			}
			path := filepath.Join(stage, failureReportName)
			info, err := os.Lstat(path)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					problems = append(problems, fmt.Errorf("inspect %s: %w", path, err))
				}
				continue
			}
			if !info.Mode().IsRegular() {
				problems = append(problems, fmt.Errorf("reject %s: %w", path, errInvalidFailureReport))
				continue
			}
			reported, err := readFailureReport(path, info)
			if err != nil {
				problems = append(problems, fmt.Errorf("read %s: %w", path, err))
				continue
			}
			if err := os.Remove(path); err != nil {
				problems = append(problems, fmt.Errorf("consume %s: %w", path, err))
			}
			if message == "" || info.ModTime().After(newest) {
				message, newest = reported, info.ModTime()
			}
		}
	}
	return message, errors.Join(problems...)
}

func validFailureStage(root, stage string) bool {
	if filepath.Dir(stage) != root {
		return false
	}
	info, err := os.Lstat(stage)
	if err != nil || !info.IsDir() {
		return false
	}
	resolved, err := resolveUpdatePath(stage)
	return err == nil && samePath(resolved, stage)
}

func readFailureReport(path string, info os.FileInfo) (string, error) {
	if info.Size() <= 0 || info.Size() > maxFailureReportSize {
		return "", fmt.Errorf("%w: invalid size", errInvalidFailureReport)
	}
	if err := safeRegular(path); err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidFailureReport, err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var report updateFailureReport
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return "", fmt.Errorf("%w: %v", errInvalidFailureReport, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", fmt.Errorf("%w: unexpected trailing data", errInvalidFailureReport)
	}
	report.Message = strings.TrimSpace(report.Message)
	if report.Message == "" {
		return "", fmt.Errorf("%w: empty message", errInvalidFailureReport)
	}
	return report.Message, nil
}
