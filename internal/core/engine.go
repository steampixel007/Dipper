package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func (c *Core) PreviewDirectory(anime Anime) (string, error) {
	if strings.TrimSpace(anime.ID) == "" {
		return "", errors.New("anime ID is required")
	}
	snap, err := c.Snapshot()
	if err != nil {
		return "", err
	}
	s := diskState{Settings: snap.Settings, Anime: map[string]AnimeRecord{}}
	for _, a := range snap.Anime {
		s.Anime[a.Anime.ID] = a
	}
	return animeDirectory(&s, anime), nil
}

func resourceKey(r Resource) string {
	if r.ID != "" {
		return r.ID
	}
	if hash, err := ResourceHash(r); err == nil {
		return hash
	}
	return r.TorrentURL
}

func (c *Core) CreateSubscription(ctx context.Context, anime Anime, group Group, rule Rule, history []Resource, selectedIDs []string) (Subscription, error) {
	var result Subscription
	if strings.TrimSpace(anime.ID) == "" || strings.TrimSpace(group.ID) == "" {
		return result, errors.New("anime and group IDs are required")
	}
	ctx, done, err := c.operation(ctx)
	if err != nil {
		return result, err
	}
	defer done()
	selected := map[string]bool{}
	for _, id := range selectedIDs {
		selected[id] = true
	}
	known := map[string]bool{}
	for _, r := range history {
		key := resourceKey(r)
		if key == "" {
			return result, errors.New("history resource has no identity")
		}
		if known[key] {
			return result, fmt.Errorf("duplicate history resource: %s", key)
		}
		known[key] = true
		if selected[key] && !Match(r.Title, rule).Matches {
			return result, fmt.Errorf("selected resource does not match rule: %s", key)
		}
	}
	for id := range selected {
		if !known[id] {
			return result, fmt.Errorf("selected resource missing from history: %s", id)
		}
	}
	id := SubscriptionID(anime.ID, group.ID)
	var creationCtx context.Context
	var creationDone func()
	err = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		if _, ok := s.Subscriptions[id]; ok {
			return ErrExists
		}
		result = Subscription{ID: id, Anime: anime, Group: group, Rule: NormalizeRule(rule), Directory: animeDirectory(s, anime), Revision: 1, CreatedAt: c.opts.Now(), NextCheck: c.opts.Now().Add(c.opts.Interval), Items: map[string]Item{}}
		for _, r := range history {
			key := resourceKey(r)
			m := Match(r.Title, result.Rule)
			item := Item{Resource: r, Read: true, Status: ItemExcluded, Reason: m.Reason}
			item.Hash, _ = ResourceHash(r)
			if m.Matches {
				item.Status = ItemSkipped
				item.Reason = "创建订阅时取消勾选"
				if selected[key] {
					item.Status = ItemPending
					item.Read = false
					item.Reason = "等待提交下载"
					item.Explicit = true
				}
			}
			if item.Status == ItemPending {
				if err := c.enqueue(ctx, tx, s, &result, &item); err != nil {
					return err
				}
			}
			result.Items[key] = item
		}
		s.Subscriptions[id] = result
		creationCtx, creationDone = c.registerLocked(ctx, id)
		return nil
	})
	if creationDone != nil {
		defer creationDone()
	}
	if err != nil {
		return Subscription{}, err
	}
	err = c.flush(creationCtx, id, false, true)
	readErr := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		if err := creationCtx.Err(); err != nil {
			return err
		}
		current, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if current.Revision != result.Revision {
			return ErrStale
		}
		result = current
		if err != nil && ctx.Err() == nil {
			result.LastError = err.Error()
			result.Failures++
			result.NextCheck = c.opts.Now().Add(c.backoff(result.Failures))
			s.Subscriptions[id] = result
		}
		return nil
	})
	if err == nil {
		err = readErr
	}
	c.signal()
	return result, err
}

func (c *Core) enqueue(ctx context.Context, tx *sql.Tx, s *diskState, sub *Subscription, item *Item) error {
	if item.Hash == "" || c.removing[item.Hash] > 0 {
		return nil
	}
	record, err := taskByHash(ctx, tx, item.Hash)
	if err == nil {
		if record.Accepted {
			item.Read = true
			item.Status = ItemDownloaded
			item.Reason = "该 BT 资源已提交"
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	resource := item.Resource
	if hash, hashErr := ResourceHash(resource); hashErr != nil || hash != item.Hash {
		resource.Magnet = "magnet:?xt=urn:btih:" + item.Hash
	}
	return saveTask(ctx, tx, TaskRecord{Task: DownloadTask{Hash: item.Hash, AnimeID: sub.Anime.ID, SubscriptionID: sub.ID, Resource: resource, Directory: filepath.Join(sub.Directory, item.Hash)}})
}

func (c *Core) UpdateRule(ctx context.Context, id string, rule Rule) error {
	err := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		sub, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		sub.Rule = NormalizeRule(rule)
		sub.Revision++
		c.cancelSubscriptionLocked(id)
		s.Subscriptions[id] = sub
		return nil
	})
	if err == nil {
		c.signal()
	}
	return err
}

func (c *Core) DownloadExisting(ctx context.Context, id string, history []Resource, selectedIDs []string) error {
	ctx, done, err := c.operation(ctx)
	if err != nil {
		return err
	}
	defer done()
	resources := map[string]Resource{}
	for _, r := range history {
		key := resourceKey(r)
		if key == "" {
			return errors.New("history resource has no identity")
		}
		if _, exists := resources[key]; exists {
			return fmt.Errorf("duplicate history resource: %s", key)
		}
		resources[key] = r
	}
	for _, key := range selectedIDs {
		if _, ok := resources[key]; !ok {
			return fmt.Errorf("selected resource missing from history: %s", key)
		}
	}
	err = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		sub, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		for _, key := range selectedIDs {
			r := resources[key]
			hash, _ := ResourceHash(r)
			if old, ok := sub.Items[key]; ok && old.Hash != "" {
				if hash != "" && old.Hash != hash {
					return fmt.Errorf("resource identity changed: %s", key)
				}
				if hash == "" {
					hash = old.Hash
					r.Magnet = "magnet:?xt=urn:btih:" + hash
				}
			}
			item := Item{Resource: r, Hash: hash, Status: ItemPending, Reason: "主动补下载，等待提交", Explicit: true}
			if err := c.enqueue(ctx, tx, s, &sub, &item); err != nil {
				return err
			}
			sub.Items[key] = item
		}
		s.Subscriptions[id] = sub
		return nil
	})
	if err != nil {
		return err
	}
	err = c.flush(ctx, id, false, true)
	c.signal()
	return err
}

func (c *Core) SetPaused(ctx context.Context, id string, paused bool) error {
	err := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		sub, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if sub.Paused != paused {
			c.cancelSubscriptionLocked(id)
			sub.Paused = paused
			sub.Revision++
			if !paused {
				sub.NextCheck = c.opts.Now()
			}
			s.Subscriptions[id] = sub
		}
		return nil
	})
	if err == nil {
		c.signal()
	}
	return err
}

// RemoveSubscription stops tracking an anime and group. Submissions the
// downloader has not accepted yet are dropped so they never start; accepted
// transfers keep running and their records stay, because Start replays them
// to restore the running engine. Files on disk are never touched.
func (c *Core) RemoveSubscription(ctx context.Context, id string) error {
	err := c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		if _, ok := s.Subscriptions[id]; !ok {
			return ErrNotFound
		}
		c.cancelSubscriptionLocked(id)
		delete(s.Subscriptions, id)
		tasks, err := readTasks(ctx, tx)
		if err != nil {
			return err
		}
		for _, r := range tasks {
			if r.Accepted || r.Independent || r.Task.SubscriptionID != id {
				continue
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM core_tasks WHERE hash=?`, r.Task.Hash); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		c.signal()
	}
	return err
}

// DeleteTask forgets a delivery and marks its pending or downloaded items
// skipped. The caller owns removing the transfer and its files.
func (c *Core) DeleteTask(ctx context.Context, hash string) error {
	return c.RemoveTask(ctx, hash, false, nil)
}

// ResetTask queues a fresh delivery. Without a surviving subscription item,
// the saved task becomes an independent intent using its original source and folder.
func (c *Core) ResetTask(ctx context.Context, hash string) error {
	return c.RemoveTask(ctx, hash, true, nil)
}

// RemoveTask cancels admissions for hash and waits for delivery to finish before
// invoking remove outside the state lock. A true result means the downloader
// forgot the task, even if deleting its files returned a cleanup error; core must
// then finalize the intent so restart cannot resurrect it. A false result keeps
// the saved intent. A nil callback only updates core's delivery state.
func (c *Core) RemoveTask(ctx context.Context, hash string, redownload bool, remove func() (removed bool, err error)) error {
	ctx, done, err := c.operation(ctx)
	if err != nil {
		return err
	}
	defer done()
	c.mu.Lock()
	c.removing[hash]++
	for op := range c.active {
		if op.hash == hash {
			op.cancel()
		}
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.removing[hash]--
		if c.removing[hash] == 0 {
			delete(c.removing, hash)
		}
		c.mu.Unlock()
	}()
	select {
	case c.delivery <- struct{}{}:
		defer func() { <-c.delivery }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		record, err := taskByHash(ctx, tx, hash)
		if err == nil {
			// The tombstone is the durable intent: a crash after this commit
			// leaves a state startup recovery completes.
			record.Removing, record.RemoveRedownload = true, redownload
			return saveTask(ctx, tx, record)
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for _, sub := range s.Subscriptions {
			for _, item := range sub.Items {
				if item.Hash == hash && (item.Status == ItemDownloaded || item.Status == ItemPending) {
					return nil
				}
			}
		}
		return ErrTaskNotFound
	}); err != nil {
		return err
	}
	var removeErr error
	if remove != nil {
		removed, err := remove()
		if !removed {
			if err == nil {
				err = errors.New("downloader did not remove the task")
			}
			return err
		}
		removeErr = err
		// Once the downloader forgot it, caller cancellation cannot leave a
		// saved accepted record that a later startup would replay.
		ctx = context.WithoutCancel(ctx)
	}
	status, reason, read := ItemSkipped, "已删除下载任务", true
	if redownload {
		status, reason, read = ItemPending, "等待重新下载", false
	}
	return errors.Join(removeErr, c.forgetTask(ctx, hash, status, reason, read))
}

func (c *Core) forgetTask(ctx context.Context, hash string, status ItemStatus, reason string, read bool) error {
	err := c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		found := false
		record, recordErr := taskByHash(ctx, tx, hash)
		if recordErr == nil {
			found = true
			if _, err := tx.ExecContext(ctx, `DELETE FROM core_tasks WHERE hash=?`, hash); err != nil {
				return err
			}
		} else if !errors.Is(recordErr, sql.ErrNoRows) {
			return recordErr
		}
		hasItem := false
		for id, sub := range s.Subscriptions {
			changed := false
			for key, item := range sub.Items {
				if item.Hash != hash || (item.Status != ItemDownloaded && item.Status != ItemPending) {
					continue
				}
				hasItem = true
				found = true
				if status == ItemPending && item.Status == ItemPending {
					continue
				}
				item.Status = status
				item.Read = read
				item.Reason = reason
				item.LastError = ""
				item.NextRetry = time.Time{}
				sub.Items[key] = item
				changed = true
			}
			if changed {
				s.Subscriptions[id] = sub
			}
		}
		if !found {
			return ErrTaskNotFound
		}
		if status == ItemPending && !hasItem && recordErr == nil {
			record.Accepted = false
			record.Independent = true
			record.Removing = false
			record.RemoveRedownload = false
			record.Attempts = 0
			record.LastError = ""
			record.NextRetry = time.Time{}
			return saveTask(ctx, tx, record)
		}
		return nil
	})
	if err == nil {
		c.signal()
	}
	return err
}

// recoverRemovals finishes removals whose two halves a crash or a downloader
// failure separated: the tombstone (TaskRecord.Removing) was committed before
// the downloader was told to forget the task, and core's records were not
// finalized yet. Recovery removes what the downloader still holds and then
// finalizes, so the stores converge no matter where the process died. A
// downloader failure backs off through the record's NextRetry.
func (c *Core) recoverRemovals(ctx context.Context, force bool) {
	snap, err := c.Snapshot()
	if err != nil {
		return
	}
	for _, record := range snap.Tasks {
		if !record.Removing {
			continue
		}
		if err := ctx.Err(); err != nil {
			return
		}
		if !force && !record.NextRetry.IsZero() && record.NextRetry.After(c.opts.Now()) {
			continue
		}
		c.mu.Lock()
		busy := c.removing[record.Task.Hash] > 0
		c.mu.Unlock()
		if busy {
			continue
		}
		if c.opts.Downloader != nil {
			if removeErr := c.opts.Downloader.Remove(record.Task.Hash); removeErr != nil {
				_ = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
					latest, err := taskByHash(ctx, tx, record.Task.Hash)
					if err != nil {
						return err
					}
					latest.LastError = removeErr.Error()
					latest.NextRetry = c.opts.Now().Add(c.backoff(latest.Attempts + 1))
					return saveTask(ctx, tx, latest)
				})
				continue
			}
		}
		status, reason, read := ItemSkipped, "已删除下载任务", true
		if record.RemoveRedownload {
			status, reason, read = ItemPending, "等待重新下载", false
		}
		_ = c.forgetTask(ctx, record.Task.Hash, status, reason, read)
	}
}

func (c *Core) getSubscription(ctx context.Context, id string) (Subscription, error) {
	var sub Subscription
	err := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		var ok bool
		sub, ok = s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		return nil
	})
	return sub, err
}

func (c *Core) Check(ctx context.Context, id string) error {
	ctx, done, err := c.operation(ctx)
	if err != nil {
		return err
	}
	defer done()
	return c.check(ctx, id)
}

func (c *Core) check(ctx context.Context, id string) error {
	sub, err := c.getSubscription(ctx, id)
	if err != nil {
		return err
	}
	if sub.Paused {
		return nil
	}
	ctx, cleanup, err := c.watchSubscription(ctx, id, sub.Revision)
	if err != nil {
		return err
	}
	defer cleanup()
	if c.opts.Feed == nil {
		return c.finishCheck(ctx, id, sub.Revision, errors.New("resource source is not configured"))
	}
	resources, err := c.opts.Feed(ctx, sub.Anime.ID, sub.Group.ID)
	if err != nil {
		return c.finishCheck(ctx, id, sub.Revision, err)
	}
	err = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		current, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if current.Paused || current.Revision != sub.Revision {
			return ErrStale
		}
		for _, r := range resources {
			key := resourceKey(r)
			if key == "" {
				return errors.New("feed resource has no identity")
			}
			if _, known := current.Items[key]; known {
				continue
			}
			hash, _ := ResourceHash(r)
			var alias *Item
			if hash != "" {
				for _, v := range current.Items {
					if v.Hash == hash {
						copy := v
						alias = &copy
						break
					}
				}
			}
			if alias != nil {
				alias.Resource = r
				current.Items[key] = *alias
				continue
			}
			m := Match(r.Title, current.Rule)
			item := Item{Resource: r, Hash: hash, Read: !m.Matches, Status: ItemExcluded, Reason: m.Reason}
			if m.Matches {
				item.Status = ItemPending
				item.Reason = "等待提交下载"
				if err := c.enqueue(ctx, tx, s, &current, &item); err != nil {
					return err
				}
			}
			current.Items[key] = item
		}
		s.Subscriptions[id] = current
		return nil
	})
	if err != nil {
		return err
	}
	err = c.flush(ctx, id, false, true)
	return c.finishCheck(ctx, id, sub.Revision, err)
}

func (c *Core) backoff(n int) time.Duration {
	d := c.opts.RetryBase
	for i := 1; i < n && d < c.opts.RetryMax; i++ {
		if d > c.opts.RetryMax/2 {
			return c.opts.RetryMax
		}
		d *= 2
	}
	if d > c.opts.RetryMax {
		return c.opts.RetryMax
	}
	return d
}

func (c *Core) finishCheck(ctx context.Context, id string, revision uint64, checkErr error) error {
	err := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		sub, ok := s.Subscriptions[id]
		if !ok {
			return ErrNotFound
		}
		if sub.Paused || sub.Revision != revision {
			return ErrStale
		}
		sub.LastCheck = c.opts.Now()
		if checkErr == nil {
			sub.LastSuccess = sub.LastCheck
			sub.LastError = ""
			sub.Failures = 0
			sub.NextCheck = sub.LastCheck.Add(c.opts.Interval)
		} else {
			sub.LastError = checkErr.Error()
			sub.Failures++
			sub.NextCheck = sub.LastCheck.Add(c.backoff(sub.Failures))
		}
		s.Subscriptions[id] = sub
		return nil
	})
	if err != nil {
		return err
	}
	return checkErr
}

// RetryPending retries saved intents without needing the source to be reachable.
func (c *Core) RetryPending(ctx context.Context) error {
	ctx, done, err := c.operation(ctx)
	if err != nil {
		return err
	}
	defer done()
	return c.flush(ctx, "", false, true)
}

func (c *Core) flush(ctx context.Context, onlyID string, replay, force bool) error {
	select {
	case c.delivery <- struct{}{}:
		defer func() { <-c.delivery }()
	case <-ctx.Done():
		return ctx.Err()
	}
	var errs []error
	snap, err := c.Snapshot()
	if err != nil {
		return err
	}
	for _, sub := range snap.Subscriptions {
		if sub.Paused || (onlyID != "" && sub.ID != onlyID) {
			continue
		}
		for key, item := range sub.Items {
			if item.Status != ItemPending {
				continue
			}
			if !force && item.Hash == "" && item.NextRetry.After(c.opts.Now()) {
				continue
			}
			hash := item.Hash
			var resolveErr error
			if hash == "" {
				resolveCtx, cleanup, watchErr := c.watchSubscription(ctx, sub.ID, sub.Revision)
				if watchErr != nil {
					errs = append(errs, watchErr)
					continue
				}
				hash, resolveErr = resolveHash(resolveCtx, item.Resource)
				cleanup()
			}
			if resolveErr != nil {
				persistErr := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
					current := s.Subscriptions[sub.ID]
					if current.Paused || current.Revision != sub.Revision {
						return ErrStale
					}
					v := current.Items[key]
					v.Attempts++
					v.LastError = resolveErr.Error()
					v.Reason = v.LastError
					v.NextRetry = c.opts.Now().Add(c.backoff(v.Attempts))
					current.Items[key] = v
					s.Subscriptions[sub.ID] = current
					return nil
				})
				if persistErr != nil {
					errs = append(errs, persistErr)
				}
				errs = append(errs, fmt.Errorf("%s: %w", key, resolveErr))
				continue
			}
			err = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
				current := s.Subscriptions[sub.ID]
				if current.Paused || current.Revision != sub.Revision {
					return ErrStale
				}
				v := current.Items[key]
				if v.Status != ItemPending {
					return nil
				}
				v.Hash = hash
				v.LastError = ""
				v.NextRetry = time.Time{}
				for _, old := range current.Items {
					if !v.Explicit && old.Hash == hash && old.Status != ItemPending {
						v.Status = old.Status
						v.Read = old.Read
						v.Reason = old.Reason
						break
					}
				}
				if v.Status == ItemPending {
					if err := c.enqueue(ctx, tx, s, &current, &v); err != nil {
						return err
					}
				}
				current.Items[key] = v
				s.Subscriptions[sub.ID] = current
				return nil
			})
			if err != nil {
				errs = append(errs, err)
			}
		}
	}
	snap, err = c.Snapshot()
	if err != nil {
		return err
	}
	for _, record := range snap.Tasks {
		if record.Removing {
			continue
		}
		if record.Accepted && !replay && record.LastError == "" {
			continue
		}
		if !force && !record.NextRetry.IsZero() && record.NextRetry.After(c.opts.Now()) {
			continue
		}
		var deliveryCtx context.Context
		var cleanup func()
		err = c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
			if record.Accepted {
				deliveryCtx = ctx
				cleanup = func() {}
				return nil
			}
			// A concurrent RemoveTask is draining this hash; do not admit it again.
			if c.removing[record.Task.Hash] > 0 {
				return nil
			}
			for _, sub := range s.Subscriptions {
				if sub.Paused || (onlyID != "" && sub.ID != onlyID) {
					continue
				}
				for _, item := range sub.Items {
					if item.Hash == record.Task.Hash && item.Status == ItemPending {
						deliveryCtx, cleanup = c.registerTaskLocked(ctx, sub.ID, record.Task.Hash)
						return nil
					}
				}
			}
			// A reset without a surviving subscription keeps its saved task as an
			// independent intent; it delivers under the caller's context alone.
			if record.Independent && onlyID == "" {
				deliveryCtx, cleanup = c.registerTaskLocked(ctx, "", record.Task.Hash)
			}
			return nil
		})
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return err
		}
		if deliveryCtx == nil {
			continue
		}
		if err := deliveryCtx.Err(); err != nil {
			cleanup()
			return err
		}
		task := record.Task
		task.Context = deliveryCtx
		var submitErr error
		if c.opts.Downloader == nil {
			submitErr = errors.New("downloader is not configured")
		} else {
			submitErr = c.opts.Downloader.Ensure(task)
		}
		if err := deliveryCtx.Err(); err != nil {
			cleanup()
			return err
		}
		err = c.transaction(deliveryCtx, func(tx *sql.Tx, s *diskState) error {
			latest, err := taskByHash(ctx, tx, task.Hash)
			if err != nil {
				return err
			}
			latest.Attempts++
			if submitErr != nil {
				latest.LastError = submitErr.Error()
				latest.NextRetry = c.opts.Now().Add(c.backoff(latest.Attempts))
				for id, sub := range s.Subscriptions {
					if sub.Paused {
						continue
					}
					for key, item := range sub.Items {
						if item.Hash == task.Hash && item.Status == ItemPending {
							item.Reason = latest.LastError
							item.LastError = latest.LastError
							item.Attempts = latest.Attempts
							item.NextRetry = latest.NextRetry
							sub.Items[key] = item
						}
					}
					s.Subscriptions[id] = sub
				}
			} else {
				latest.Accepted = true
				latest.LastError = ""
				latest.NextRetry = time.Time{}
				for id, sub := range s.Subscriptions {
					if sub.Paused {
						continue
					}
					for key, item := range sub.Items {
						if item.Hash == task.Hash && item.Status == ItemPending {
							item.Read = true
							item.Status = ItemDownloaded
							item.Reason = "已提交下载"
							item.LastError = ""
							item.NextRetry = time.Time{}
							sub.Items[key] = item
						}
					}
					s.Subscriptions[id] = sub
				}
			}
			return saveTask(ctx, tx, latest)
		})
		cleanup()
		if err != nil {
			return err
		}
		if submitErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", task.Hash, submitErr))
		}
	}
	return errors.Join(errs...)
}

func (c *Core) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	if c.started {
		return nil
	}
	c.started = true
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.recoverRemovals(c.ctx, true)
		_ = c.flush(c.ctx, "", true, true)
		if c.opts.DisableScheduler {
			return
		}
		c.schedule()
	}()
	return nil
}

func (c *Core) schedule() {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.wake:
		case <-timer.C:
		}
		if c.ctx.Err() != nil {
			return
		}
		snap, err := c.Snapshot()
		if err != nil {
			return
		}
		now := c.opts.Now()
		for _, sub := range snap.Subscriptions {
			if !sub.Paused && !sub.NextCheck.After(now) {
				_ = c.check(c.ctx, sub.ID)
			}
			if c.ctx.Err() != nil {
				return
			}
		}
		c.recoverRemovals(c.ctx, false)
		_ = c.flush(c.ctx, "", false, false)
		snap, err = c.Snapshot()
		if err != nil {
			return
		}
		delay := c.opts.Interval
		for _, sub := range snap.Subscriptions {
			if !sub.Paused {
				d := sub.NextCheck.Sub(c.opts.Now())
				if d < delay {
					delay = d
				}
				for _, item := range sub.Items {
					if item.Status == ItemPending && item.Hash == "" && !item.NextRetry.IsZero() {
						d := item.NextRetry.Sub(c.opts.Now())
						if d < delay {
							delay = d
						}
					}
				}
			}
		}
		for _, r := range snap.Tasks {
			if r.Removing {
				continue
			}
			if r.LastError != "" {
				active := r.Accepted || r.Independent
				for _, sub := range snap.Subscriptions {
					if sub.Paused {
						continue
					}
					for _, item := range sub.Items {
						if item.Hash == r.Task.Hash && item.Status == ItemPending {
							active = true
							break
						}
					}
				}
				if active {
					d := r.NextRetry.Sub(c.opts.Now())
					if d < delay {
						delay = d
					}
				}
			}
		}
		if delay < 10*time.Millisecond {
			delay = 10 * time.Millisecond
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(delay)
	}
}
