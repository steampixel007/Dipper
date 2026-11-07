package downloader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/anacrolix/torrent/metainfo"
)

type ownedFile struct {
	Path   string `json:"path"`
	Length int64  `json:"length"`
}

type fileClaim struct {
	Folder string      `json:"folder"`
	Hash   string      `json:"hash"`
	Files  []ownedFile `json:"files"`
}

type ownership struct {
	mu     sync.Mutex
	path   string
	claims map[string]fileClaim
}

func openOwnership(dir string) (*ownership, error) {
	o := &ownership{path: filepath.Join(dir, "files.json"), claims: make(map[string]fileClaim)}
	body, err := os.ReadFile(o.path)
	if errors.Is(err, os.ErrNotExist) {
		return o, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &o.claims); err != nil || o.claims == nil {
		return nil, fmt.Errorf("invalid file ownership checkpoint: %v", err)
	}
	for id, claim := range o.claims {
		if id == "" || claim.Hash == "" || !filepath.IsAbs(claim.Folder) || overlappingFolders(dir, claim.Folder) {
			return nil, errors.New("invalid file ownership record")
		}
		if err := o.conflicts(id, claim); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func torrentFiles(info *metainfo.Info) ([]diskFile, error) {
	if err := safeComponent(info.BestName()); err != nil {
		return nil, err
	}
	var files []diskFile
	for _, file := range info.UpvertedFiles() {
		parts := append([]string{info.BestName()}, file.BestPath()...)
		for _, part := range parts {
			if err := safeComponent(part); err != nil {
				return nil, err
			}
		}
		if file.Length < 0 || file.TorrentOffset < 0 {
			return nil, errors.New("torrent contains a negative file extent")
		}
		files = append(files, diskFile{filepath.Join(parts...), file.TorrentOffset, file.Length})
	}
	claim := fileClaim{Folder: string(filepath.Separator), Files: ownedFiles(files)}
	if err := validateClaims([]fileClaim{claim}); err != nil {
		return nil, err
	}
	return files, nil
}

func ownedFiles(files []diskFile) []ownedFile {
	result := make([]ownedFile, 0, len(files))
	for _, file := range files {
		result = append(result, ownedFile{file.name, file.length})
	}
	return result
}

func validateClaims(claims []fileClaim) error {
	type pathUse struct {
		name string
		file bool
	}
	used := make(map[string]pathUse)
	for _, claim := range claims {
		for _, file := range claim.Files {
			if file.Length < 0 || !filepath.IsLocal(file.Path) {
				return errors.New("invalid owned file")
			}
			parts := strings.Split(file.Path, string(filepath.Separator))
			for _, part := range parts {
				if err := safeComponent(part); err != nil {
					return err
				}
			}
			for index := range parts {
				name := filepath.Join(claim.Folder, filepath.Join(parts[:index+1]...))
				key := strings.ToLower(name)
				isFile := index == len(parts)-1
				if prior, ok := used[key]; ok && (prior.name != name || prior.file || isFile) {
					return fmt.Errorf("colliding file path %q", name)
				}
				used[key] = pathUse{name, isFile}
			}
		}
	}
	return nil
}

func (o *ownership) conflicts(id string, claim fileClaim) error {
	claims := []fileClaim{claim}
	for otherID, other := range o.claims {
		if otherID != id {
			claims = append(claims, other)
		}
	}
	return validateClaims(claims)
}

func (o *ownership) save() error {
	body, err := json.MarshalIndent(o.claims, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(o.path, body)
}

func (o *ownership) reserve(task Task, hash string, files []diskFile) error {
	claim := fileClaim{Folder: task.Folder, Hash: hash, Files: ownedFiles(files)}
	previous, exists := o.claims[task.ID]
	if exists && !reflect.DeepEqual(previous, claim) {
		return errors.New("task file ownership differs from saved metadata")
	}
	if err := o.conflicts(task.ID, claim); err != nil {
		return err
	}
	root, err := checkedRoot(task.Folder)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, file := range files {
		if err := checkPath(root, file.name); err != nil {
			return err
		}
		stat, err := root.Lstat(file.name)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if task.Shared && !exists {
			return fmt.Errorf("unowned file already exists: %s", file.name)
		}
		if !stat.Mode().IsRegular() || stat.Size() > file.length {
			return fmt.Errorf("existing file has incompatible type or size: %s", file.name)
		}
		f, err := openPayload(root, file.name, os.O_RDONLY)
		if err != nil {
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
	if exists {
		return nil
	}
	o.claims[task.ID] = claim
	if err := o.save(); err != nil {
		delete(o.claims, task.ID)
		return fmt.Errorf("save file ownership: %w", err)
	}
	return nil
}

func checkedRoot(folder string) (*os.Root, error) {
	if err := checkSavedFolder(folder); err != nil {
		return nil, err
	}
	return os.OpenRoot(folder)
}

func checkFolder(folder string) error {
	stat, err := os.Lstat(folder)
	if err != nil {
		return err
	}
	if isReparse(stat) || !stat.IsDir() {
		return fmt.Errorf("unsafe content directory: %s", folder)
	}
	return nil
}

func checkPath(root *os.Root, name string) error {
	if !filepath.IsLocal(name) {
		return fmt.Errorf("unsafe content path: %s", name)
	}
	parts := strings.Split(name, string(filepath.Separator))
	parent := "."
	for index, part := range parts {
		if err := safeComponent(part); err != nil {
			return err
		}
		dir, err := root.Open(parent)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		entries, readErr := dir.ReadDir(-1)
		closeErr := dir.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) && entry.Name() != part {
				return fmt.Errorf("case-insensitive content collision: %s", name)
			}
		}
		parent = filepath.Join(parent, part)
		stat, err := root.Lstat(parent)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if isReparse(stat) || index < len(parts)-1 && !stat.IsDir() {
			return fmt.Errorf("unsafe content path: %s", name)
		}
	}
	return nil
}

func openPayload(root *os.Root, name string, flags int) (*os.File, error) {
	if err := checkPath(root, name); err != nil {
		return nil, err
	}
	stat, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsRegular() {
		return nil, fmt.Errorf("nonregular payload: %s", name)
	}
	file, err := root.OpenFile(name, flags, 0)
	if err != nil {
		return nil, err
	}
	if err := singleLink(file); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (o *ownership) remove(task Task) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	claim, exists := o.claims[task.ID]
	if !exists {
		return nil
	}
	if claim.Folder != task.Folder {
		return errors.New("removal folder differs from owned files")
	}
	root, err := checkedRoot(task.Folder)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer root.Close()
	dirs := make(map[string]bool)
	var failures []error
	for _, file := range claim.Files {
		// A payload the user already deleted still leaves its directories behind.
		for dir := filepath.Dir(file.Path); dir != "."; dir = filepath.Dir(dir) {
			dirs[dir] = true
		}
		f, err := openPayload(root, file.Path, os.O_RDONLY)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err == nil {
			err = f.Close()
			if err == nil {
				err = root.Remove(file.Path)
			}
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	names := make([]string, 0, len(dirs))
	for dir := range dirs {
		names = append(names, dir)
	}
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, dir := range names {
		if err := checkPath(root, dir); err == nil {
			_ = root.Remove(dir)
		}
	}
	if err := errors.Join(failures...); err != nil {
		return err
	}
	return nil
}

func (o *ownership) release(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	claim, exists := o.claims[id]
	if !exists {
		return nil
	}
	delete(o.claims, id)
	if err := o.save(); err != nil {
		o.claims[id] = claim
		return err
	}
	return nil
}

func (o *ownership) files(task Task) ([]ownedFile, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	claim, ok := o.claims[task.ID]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if claim.Folder != task.Folder {
		return nil, errors.New("task folder differs from owned files")
	}
	return append([]ownedFile(nil), claim.Files...), nil
}
