package downloader

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

type taskFailure struct{ err error }

type diskStorage struct {
	folder     string
	completion storage.PieceCompletion
	failure    *atomic.Pointer[taskFailure]
}

type diskFile struct {
	name           string
	offset, length int64
}

type diskTorrent struct {
	root       *os.Root
	files      []diskFile
	completion storage.PieceCompletion
	hash       metainfo.Hash
	failure    *atomic.Pointer[taskFailure]
}

func (s *diskStorage) OpenTorrent(_ context.Context, info *metainfo.Info, hash metainfo.Hash) (storage.TorrentImpl, error) {
	d, err := s.open(info, hash)
	if err != nil {
		s.failure.Store(&taskFailure{err})
		return storage.TorrentImpl{}, err
	}
	return storage.TorrentImpl{Piece: d.piece, Close: d.root.Close}, nil
}

func (s *diskStorage) open(info *metainfo.Info, hash metainfo.Hash) (_ *diskTorrent, err error) {
	if err = safeComponent(info.BestName()); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(s.folder)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			root.Close()
		}
	}()
	d := &diskTorrent{root: root, completion: s.completion, hash: hash, failure: s.failure}
	names := make(map[string]bool)
	for _, file := range info.UpvertedFiles() {
		parts := append([]string{info.BestName()}, file.BestPath()...)
		for _, part := range parts {
			if err = safeComponent(part); err != nil {
				return nil, err
			}
		}
		name := filepath.Join(parts...)
		key := strings.ToLower(name)
		if names[key] {
			return nil, fmt.Errorf("torrent contains colliding file path %q", name)
		}
		names[key] = true
		if file.Length < 0 || file.TorrentOffset < 0 {
			return nil, errors.New("torrent contains a negative file extent")
		}
		if err = root.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			return nil, err
		}
		f, openErr := root.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o644)
		if openErr != nil {
			return nil, openErr
		}
		stat, statErr := f.Stat()
		closeErr := f.Close()
		if statErr != nil {
			return nil, statErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if !stat.Mode().IsRegular() || stat.Size() > file.Length {
			return nil, fmt.Errorf("existing file has incompatible type or size: %s", name)
		}
		d.files = append(d.files, diskFile{name, file.TorrentOffset, file.Length})
	}
	return d, nil
}

func safeComponent(s string) error {
	if s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\:\x00<>\"|?*") || strings.TrimRight(s, " .") != s {
		return fmt.Errorf("unsafe torrent path component %q", s)
	}
	stem := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return fmt.Errorf("reserved torrent path component %q", s)
	}
	return nil
}

func (d *diskTorrent) piece(p metainfo.Piece) storage.PieceImpl {
	return &diskPiece{diskTorrent: d, pieceInfo: p}
}

type diskPiece struct {
	*diskTorrent
	pieceInfo metainfo.Piece
}

func (p *diskPiece) ReadAt(b []byte, off int64) (int, error) {
	return p.transfer(b, off, false)
}

func (p *diskPiece) WriteAt(b []byte, off int64) (int, error) {
	n, err := p.transfer(b, off, true)
	if err != nil {
		p.failure.Store(&taskFailure{err})
	}
	return n, err
}

func (p *diskPiece) transfer(b []byte, off int64, write bool) (int, error) {
	if off < 0 || off > p.pieceInfo.Length() {
		return 0, io.ErrUnexpectedEOF
	}
	truncated := int64(len(b)) > p.pieceInfo.Length()-off
	if truncated {
		if write {
			return 0, io.ErrShortWrite
		}
		b = b[:p.pieceInfo.Length()-off]
	}
	pos := p.pieceInfo.Offset() + off
	n := 0
	for _, file := range p.files {
		if len(b) == 0 {
			break
		}
		if pos < file.offset || pos >= file.offset+file.length {
			continue
		}
		chunk := b[:min(int64(len(b)), file.offset+file.length-pos)]
		flags := os.O_RDONLY
		if write {
			flags = os.O_RDWR
		}
		f, err := p.root.OpenFile(file.name, flags, 0)
		if err != nil {
			return n, err
		}
		var count int
		if write {
			count, err = f.WriteAt(chunk, pos-file.offset)
		} else {
			count, err = f.ReadAt(chunk, pos-file.offset)
		}
		err = errors.Join(err, f.Close())
		n += count
		if err != nil {
			return n, err
		}
		pos += int64(count)
		b = b[count:]
	}
	if len(b) != 0 {
		return n, io.ErrUnexpectedEOF
	}
	if truncated {
		return n, io.EOF
	}
	return n, nil
}

func (p *diskPiece) key() metainfo.PieceKey {
	return metainfo.PieceKey{InfoHash: p.hash, Index: p.pieceInfo.Index()}
}

func (p *diskPiece) MarkComplete() (result error) {
	defer func() {
		if result != nil {
			p.failure.Store(&taskFailure{result})
		}
	}()
	// Persist content before publishing its completion checkpoint.
	start, end := p.pieceInfo.Offset(), p.pieceInfo.Offset()+p.pieceInfo.Length()
	for _, file := range p.files {
		if file.offset >= end || file.offset+file.length <= start {
			continue
		}
		f, err := p.root.OpenFile(file.name, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		if err = errors.Join(f.Sync(), f.Close()); err != nil {
			return err
		}
	}
	return p.completion.Set(p.key(), true)
}

func (p *diskPiece) MarkNotComplete() error {
	err := p.completion.Set(p.key(), false)
	if err != nil {
		p.failure.Store(&taskFailure{err})
	}
	return err
}

func (p *diskPiece) Completion() storage.Completion {
	c, err := p.completion.Get(p.key())
	c.Err = errors.Join(c.Err, err)
	if c.Err != nil {
		p.failure.Store(&taskFailure{c.Err})
	}
	return c
}
