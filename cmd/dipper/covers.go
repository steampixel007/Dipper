package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/steampixel007/Dipper/internal/source"
	"golang.org/x/image/draw"
)

const (
	coverMemoryCap     = 128 << 20
	coverDiskCap       = 96 << 20
	coverFileCap       = 5 << 20
	coverPixelCap      = 16 << 20
	coverBitmapEdge    = 512
	coverLookbehind    = 900
	coverLookahead     = 900
	coverProbeLimit    = source.MaxConcurrentCovers
	coverDecodeLimit   = 2
	coverPrefetchLimit = source.MaxConcurrentCovers - 1
)

var coverOrder []string

type coverStage uint8

const (
	coverProbe coverStage = iota
	coverMiss
	coverFailed
)

type coverLoader struct {
	featured, visible, nearby []string
	wanted                    []string
	next                      []string
	keep                      map[string]bool
	ratios                    map[string]float32
	stages                    map[string]coverStage
	probes                    int
	fetching                  map[string]*coverFlight
	pauseUntil                time.Time
	resume                    *time.Timer
	decodeGate                chan struct{}
	disk                      coverDisk
}

type coverFlight struct {
	start  atomic.Uint32
	cancel context.CancelFunc
}

func (loader *coverLoader) isVisible(key string) bool {
	return slices.Contains(loader.featured, key) || slices.Contains(loader.visible, key)
}

const (
	coverQueued uint32 = iota
	coverStarted
	coverDecoding
	coverWithdrawn
)

func coverURL(raw string) string {
	if strings.ContainsAny(raw, "?#") {
		return source.OriginalCoverURL(raw)
	}
	return raw
}

func cachedCover(key string) *ui.Bitmap {
	coversMu.Lock()
	defer coversMu.Unlock()
	return covers[key]
}

func coverCacheKey(key string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

func (s *State) syncCovers() {
	loader := &s.coverLoader
	if s.Source == nil || s.Ctx == nil || s.Ctx.Err() != nil {
		return
	}
	if loader.stages == nil {
		loader.stages = make(map[string]coverStage)
		loader.keep = make(map[string]bool)
		loader.ratios = make(map[string]float32)
	}
	if loader.decodeGate == nil {
		loader.decodeGate = make(chan struct{}, coverDecodeLimit)
	}
	loader.next = loader.next[:0]
	for _, keys := range [][]string{loader.featured, loader.visible, loader.nearby} {
		for _, key := range keys {
			if key != "" && !slices.Contains(loader.next, key) {
				loader.next = append(loader.next, key)
			}
		}
	}
	if !slices.Equal(loader.wanted, loader.next) {
		loader.wanted, loader.next = loader.next, loader.wanted
		clear(loader.keep)
		for _, key := range loader.wanted {
			loader.keep[key] = true
			if cachedCover(key) != nil {
				touchCover(key)
			}
		}
		for key, stage := range loader.stages {
			if !loader.keep[key] && stage != coverProbe {
				delete(loader.stages, key)
			}
		}
		for key, flight := range loader.fetching {
			if !loader.keep[key] && (len(loader.wanted) == 0 || flight.start.CompareAndSwap(coverQueued, coverWithdrawn)) {
				flight.cancel()
			}
		}
		for key := range loader.ratios {
			if !loader.keep[key] && cachedCover(key) == nil {
				delete(loader.ratios, key)
			}
		}
		s.preemptOffscreenCovers()
		evictCovers(loader.keep)
	}
	s.pumpCovers()
}

// preemptOffscreenCovers gives newly visible misses the next rate slots. It
// runs when demand changes so a queued prefetch cannot start while a visible
// cover is still probing disk.
func (s *State) preemptOffscreenCovers() {
	loader := &s.coverLoader
	urgent := false
	for _, keys := range [][]string{loader.featured, loader.visible} {
		for _, key := range keys {
			if cachedCover(key) == nil && loader.fetching[key] == nil && loader.stages[key] != coverFailed {
				urgent = true
				break
			}
		}
		if urgent {
			break
		}
	}
	if !urgent {
		return
	}
	for key, flight := range loader.fetching {
		if loader.isVisible(key) {
			continue
		}
		if flight.start.CompareAndSwap(coverQueued, coverWithdrawn) {
			flight.cancel()
		}
	}
}

func (s *State) retryVisibleCovers() {
	loader := &s.coverLoader
	loader.pauseUntil = time.Time{}
	if loader.resume != nil {
		loader.resume.Stop()
		loader.resume = nil
	}
	for key, stage := range loader.stages {
		if stage == coverFailed {
			delete(loader.stages, key)
		}
	}
}

func (s *State) pumpCovers() {
	loader := &s.coverLoader
	if s.Ctx.Err() != nil {
		return
	}
	for _, key := range loader.wanted {
		if cachedCover(key) != nil || loader.fetching[key] != nil {
			continue
		}
		if _, scheduled := loader.stages[key]; !scheduled && loader.probes < coverProbeLimit {
			s.probeCover(key)
		}
	}
	fetch := ""
	for _, key := range loader.wanted {
		if cachedCover(key) != nil || loader.fetching[key] != nil {
			continue
		}
		stage, scheduled := loader.stages[key]
		if !scheduled || stage == coverProbe {
			if fetch == "" {
				return
			}
			break
		}
		if stage == coverMiss {
			fetch = key
			break
		}
	}
	if fetch == "" {
		return
	}
	if !loader.isVisible(fetch) && loader.hasPendingVisible() {
		return
	}
	if delay := time.Until(loader.pauseUntil); delay > 0 {
		for key, flight := range loader.fetching {
			if !loader.keep[key] && flight.start.CompareAndSwap(coverQueued, coverWithdrawn) {
				flight.cancel()
			}
		}
		if loader.resume == nil {
			update, ctx := s.Update, s.Ctx
			loader.resume = time.AfterFunc(delay, func() {
				if ctx.Err() == nil {
					update(func() {
						loader.resume = nil
						s.pumpCovers()
					})
				}
			})
		}
		return
	}
	prefetches := 0
	active := 0
	for key, flight := range loader.fetching {
		state := flight.start.Load()
		if state == coverQueued || state == coverStarted {
			active++
			if !loader.isVisible(key) {
				prefetches++
			}
		}
	}
	visible := loader.isVisible(fetch)
	if visible {
		for key, flight := range loader.fetching {
			if loader.isVisible(key) {
				continue
			}
			if flight.start.CompareAndSwap(coverQueued, coverWithdrawn) {
				flight.cancel()
			}
		}
	}
	if active >= source.MaxConcurrentCovers {
		if !visible {
			return
		}
		for key, flight := range loader.fetching {
			if !loader.isVisible(key) {
				if flight.start.CompareAndSwap(coverStarted, coverWithdrawn) {
					flight.cancel()
					break
				}
			}
		}
	}
	if !visible && prefetches >= coverPrefetchLimit {
		return
	}
	s.fetchCover(fetch)
}

// hasPendingVisible reports whether a visible cover still needs the next
// admission. A queued visible flight may not have reached the source limiter
// yet, so allowing a prefetch to join it can let the prefetch win the slot.
func (loader *coverLoader) hasPendingVisible() bool {
	for _, keys := range [][]string{loader.featured, loader.visible} {
		for _, key := range keys {
			if cachedCover(key) != nil || loader.stages[key] == coverFailed {
				continue
			}
			if flight := loader.fetching[key]; flight == nil || flight.start.Load() == coverQueued {
				return true
			}
		}
	}
	return false
}

func (s *State) probeCover(key string) {
	loader := &s.coverLoader
	loader.stages[key] = coverProbe
	loader.probes++
	dir, update, ctx := s.CoverDir, s.Update, s.Ctx
	go func() {
		data, ok := loader.disk.read(dir, coverCacheKey(key))
		var bitmap *ui.Bitmap
		if ok {
			bitmap, _ = s.decodeCoverData(ctx, data)
		}
		update(func() {
			loader.probes--
			delete(loader.stages, key)
			if bitmap != nil {
				s.admitCover(key, bitmap)
			} else if loader.keep[key] {
				loader.stages[key] = coverMiss
			}
			s.pumpCovers()
		})
	}()
}

func (s *State) fetchCover(key string) {
	loader := &s.coverLoader
	ctx, cancel := context.WithCancel(s.Ctx)
	flight := &coverFlight{cancel: cancel}
	if loader.fetching == nil {
		loader.fetching = make(map[string]*coverFlight)
	}
	loader.fetching[key] = flight
	delete(loader.stages, key)
	dir, client, update := s.CoverDir, s.Source, s.Update
	keep := slices.Clone(loader.wanted)
	go func() {
		defer cancel()
		data, err := client.CoverWhenReady(ctx, key, func() bool {
			if !flight.start.CompareAndSwap(coverQueued, coverStarted) {
				return false
			}
			go update(s.pumpCovers)
			return true
		})
		var bitmap *ui.Bitmap
		if err == nil {
			if flight.start.CompareAndSwap(coverStarted, coverDecoding) {
				go update(s.pumpCovers)
			}
			bitmap, err = s.decodeCoverData(ctx, data)
			if err == nil {
				loader.disk.write(dir, coverCacheKey(key), data, keep...)
			}
		}
		update(func() {
			delete(loader.fetching, key)
			if wait := source.CoverRetryDelay(err); wait > 0 {
				if until := time.Now().Add(wait); until.After(loader.pauseUntil) {
					loader.pauseUntil = until
				}
			}
			if bitmap != nil {
				s.admitCover(key, bitmap)
			} else if loader.keep[key] {
				switch {
				case errors.Is(err, context.Canceled):
					loader.stages[key] = coverMiss
				case errors.Is(err, context.DeadlineExceeded):
					loader.stages[key] = coverMiss
				case recoverableCoverError(err):
					loader.stages[key] = coverMiss
				default:
					loader.stages[key] = coverFailed
				}
			}
			s.pumpCovers()
		})
	}()
}

func recoverableCoverError(err error) bool {
	var status *source.HTTPError
	if !errors.As(err, &status) {
		return false
	}
	return status.StatusCode == 429 || status.StatusCode >= 500
}

func (s *State) decodeCoverData(ctx context.Context, data []byte) (*ui.Bitmap, error) {
	gate := s.coverLoader.decodeGate
	if gate == nil {
		return decodeCover(data)
	}
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-gate }()
	return decodeCover(data)
}

func (s *State) admitCover(key string, bitmap *ui.Bitmap) {
	if s.coverLoader.keep == nil {
		s.coverLoader.keep = make(map[string]bool)
	}
	if s.coverLoader.ratios == nil {
		s.coverLoader.ratios = make(map[string]float32)
	}
	evictCoversReserve(s.coverLoader.keep, coverBitmapBytes(bitmap))
	coversMu.Lock()
	covers[key] = bitmap
	coversMu.Unlock()
	touchCover(key)
	width, height := bitmap.Size()
	s.coverLoader.ratios[key] = float32(height) / float32(width)
}

func decodeCover(data []byte) (*ui.Bitmap, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > coverPixelCap/config.Height {
		return nil, source.ErrResponseTooLarge
	}
	edge := max(config.Width, config.Height)
	if format == "jpeg" || edge <= coverBitmapEdge {
		return ui.DecodeBitmap(data)
	}
	original, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	width := max(1, (config.Width*coverBitmapEdge+edge/2)/edge)
	height := max(1, (config.Height*coverBitmapEdge+edge/2)/edge)
	scaled := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), original, original.Bounds(), draw.Src, nil)
	return ui.NewBitmap(scaled), nil
}

func touchCover(key string) {
	coversMu.Lock()
	defer coversMu.Unlock()
	for index, used := range coverOrder {
		if used == key {
			coverOrder = append(coverOrder[:index], coverOrder[index+1:]...)
			break
		}
	}
	coverOrder = append(coverOrder, key)
}

func coverBitmapBytes(bitmap *ui.Bitmap) int64 {
	width, height := bitmap.Size()
	return int64(width) * int64(height) * 16 / 3
}

func evictCovers(keep map[string]bool) {
	evictCoversReserve(keep, 0)
}

func evictCoversReserve(keep map[string]bool, reserve int64) {
	coversMu.Lock()
	defer coversMu.Unlock()
	var total int64
	for _, key := range coverOrder {
		if bitmap := covers[key]; bitmap != nil {
			total += coverBitmapBytes(bitmap)
		}
	}
	retained := coverOrder[:0]
	for _, key := range coverOrder {
		bitmap := covers[key]
		if bitmap == nil {
			continue
		}
		if total+reserve > coverMemoryCap && !keep[key] {
			total -= coverBitmapBytes(bitmap)
			delete(covers, key)
			continue
		}
		retained = append(retained, key)
	}
	coverOrder = retained
}
