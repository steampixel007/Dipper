package downloader

import (
	"sort"
	"time"
)

const DefaultDownloadLimit = 3
const idleDownloadLimit = 10 * time.Minute

type transferMode uint8

const (
	transferWaiting transferMode = iota
	transferDownloading
	transferSeeding
)

func effectiveDownloadLimit(limit int) int {
	if limit <= 0 {
		return DefaultDownloadLimit
	}
	return limit
}

func (e *Engine) SetDownloadLimit(limit int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.closed {
		e.downloadLimit = effectiveDownloadLimit(limit)
		e.scheduleLocked(false)
	}
}

func (e *Engine) enqueueLocked(task *entry) {
	e.queueTail++
	task.queueOrder = e.queueTail
}

func (e *Engine) orderedTasksLocked() []*entry {
	tasks := make([]*entry, 0, len(e.tasks))
	for _, task := range e.tasks {
		tasks = append(tasks, task)
	}
	sort.Slice(tasks, func(left, right int) bool {
		return tasks[left].queueOrder < tasks[right].queueOrder
	})
	return tasks
}

func (e *Engine) scheduleLocked(rotateIdle bool) {
	if e.closed {
		return
	}
	var incomplete []*entry
	for _, task := range e.orderedTasksLocked() {
		switch {
		case task.torrent == nil || task.err != nil || task.Policy.Paused || task.Policy.ManuallyStopped || task.Policy.AutoStopped:
			task.setTransferLocked(transferWaiting)
		case task.Complete:
			task.setTransferLocked(transferSeeding)
		default:
			incomplete = append(incomplete, task)
		}
	}
	slots := min(e.downloadLimit, len(incomplete))
	if rotateIdle && len(incomplete) > slots {
		waiting := len(incomplete) - slots
		var rotated []*entry
		keep := 0
		for index, task := range incomplete {
			if index < slots && len(rotated) < waiting && task.idleDownload >= idleDownloadLimit {
				e.enqueueLocked(task)
				rotated = append(rotated, task)
			} else {
				incomplete[keep] = task
				keep++
			}
		}
		copy(incomplete[keep:], rotated)
	}
	for _, task := range incomplete[slots:] {
		task.setTransferLocked(transferWaiting)
	}
	for _, task := range incomplete[:slots] {
		task.setTransferLocked(transferDownloading)
	}
}

func (task *entry) setTransferLocked(mode transferMode) {
	if task.torrent == nil {
		return
	}
	if task.transfer != mode {
		if mode == transferDownloading {
			task.torrent.AllowDataDownload()
		} else if task.transfer == transferDownloading {
			task.torrent.DisallowDataDownload()
		}
		if mode == transferWaiting {
			task.torrent.DisallowDataUpload()
			task.downSpeed, task.upSpeed = 0, 0
		} else if task.transfer == transferWaiting {
			task.torrent.AllowDataUpload()
		}
		task.transfer = mode
		task.idleDownload = 0
		task.previous.downloadEligible = false
	}
	if mode == transferDownloading && !task.requested && task.torrent.Info() != nil {
		task.torrent.DownloadAll()
		task.requested = true
	}
}
