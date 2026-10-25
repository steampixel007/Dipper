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
		return nil
	})
	if err != nil {
		return Subscription{}, err
	}
	err = c.flush(ctx, id, false, true)
	readErr := c.transaction(ctx, func(_ *sql.Tx, s *diskState) error {
		result = s.Subscriptions[id]
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
	if item.Hash == "" {
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
			if r.Accepted || r.Task.SubscriptionID != id {
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

// DeleteTask forgets a delivery the user no longer wants. The outbox record is
// dropped — otherwise Start's replay would submit it again — and every pending
// or downloaded item carrying the hash is marked skipped, so neither the
// scheduler nor a later check brings it back. Excluded items keep their
// exclusion: they never asked for this download. Removing the transfer itself,
// including its files, belongs to the downloader's Remove; core never touches
// the disk.
func (c *Core) DeleteTask(ctx context.Context, hash string) error {
	return c.forgetTask(ctx, hash, ItemSkipped, "已删除下载任务", true)
}

// ResetTask forgets a delivery the user wants again. The outbox record is
// dropped and downloaded items carrying the hash return to pending, so the
// next flush submits a fresh download. Items already waiting to submit stay
// as they are, and excluded items keep their exclusion.
func (c *Core) ResetTask(ctx context.Context, hash string) error {
	return c.forgetTask(ctx, hash, ItemPending, "等待重新下载", false)
}

func (c *Core) forgetTask(ctx context.Context, hash string, status ItemStatus, reason string, read bool) error {
	err := c.transaction(ctx, func(tx *sql.Tx, s *diskState) error {
		found := false
		if _, err := taskByHash(ctx, tx, hash); err == nil {
			found = true
			if _, err := tx.ExecContext(ctx, `DELETE FROM core_tasks WHERE hash=?`, hash); err != nil {
				return err
			}
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for id, sub := range s.Subscriptions {
			changed := false
			for key, item := range sub.Items {
				wanted := item.Status == ItemDownloaded ||
					(status == ItemSkipped && item.Status == ItemPending)
				if item.Hash != hash || !wanted {
					continue
				}
				item.Status = status
				item.Read = read
				item.Reason = reason
				item.LastError = ""
				item.NextRetry = time.Time{}
				sub.Items[key] = item
				changed = true
				found = true
			}
			if changed {
				s.Subscriptions[id] = sub
			}
		}
		if !found {
			return ErrTaskNotFound
		}
		return nil
	})
	if err == nil {
		c.signal()
	}
	return err
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
			for _, sub := range s.Subscriptions {
				if sub.Paused || (onlyID != "" && sub.ID != onlyID) {
					continue
				}
				for _, item := range sub.Items {
					if item.Hash == record.Task.Hash && item.Status == ItemPending {
						deliveryCtx, cleanup = c.registerLocked(ctx, sub.ID)
						return nil
					}
				}
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
			if r.LastError != "" {
				active := r.Accepted
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
