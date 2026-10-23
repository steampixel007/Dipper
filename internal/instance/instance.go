// Package instance gives each data directory one owner and forwards later launches.
package instance

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type endpoint struct {
	Address string `json:"address"`
	Token   string `json:"token"`
	PID     int    `json:"pid"`
}

type Instance struct {
	file        *os.File
	listener    net.Listener
	endpoint    endpoint
	path        string
	activations chan struct{}
	once        sync.Once
	closeErr    error
}

// Acquire returns nil after an existing owner acknowledges activation. It never
// falls through to opening the application's databases when activation fails.
func Acquire(ctx context.Context, dir string) (*Instance, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "instance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "instance.json")
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, fmt.Errorf("Dipper 已在后台运行，但未能唤醒窗口，请稍后重试：%w", err)
		}
		locked, err := tryLock(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("锁定应用数据目录：%w", err)
		}
		if locked {
			return own(file, path)
		}
		if activate(ctx, path) == nil {
			file.Close()
			return nil, nil
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}

func own(file *os.File, path string) (*Instance, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		file.Close()
		return nil, err
	}
	var token [32]byte
	if _, err = rand.Read(token[:]); err != nil {
		listener.Close()
		file.Close()
		return nil, err
	}
	i := &Instance{file: file, listener: listener, path: path, endpoint: endpoint{Address: listener.Addr().String(), Token: hex.EncodeToString(token[:]), PID: os.Getpid()}, activations: make(chan struct{}, 1)}
	body, _ := json.Marshal(i.endpoint)
	err = os.WriteFile(path+".tmp", body, 0600)
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	if err != nil {
		i.Close()
		return nil, err
	}
	go i.serve()
	return i, nil
}

func activate(ctx context.Context, path string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var target endpoint
	if err := json.Unmarshal(body, &target); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(target.Address)
	if err != nil || host != "127.0.0.1" || len(target.Token) != 64 {
		return errors.New("invalid instance endpoint")
	}
	conn, err := (&net.Dialer{Timeout: 200 * time.Millisecond}).DialContext(ctx, "tcp4", target.Address)
	if err != nil {
		return err
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	if limit, ok := ctx.Deadline(); ok && limit.Before(deadline) {
		deadline = limit
	}
	_ = conn.SetDeadline(deadline)
	grantForeground(target.PID)
	if err := json.NewEncoder(conn).Encode(target.Token); err != nil {
		return err
	}
	var acknowledged bool
	if err := json.NewDecoder(io.LimitReader(conn, 128)).Decode(&acknowledged); err != nil {
		return err
	}
	if !acknowledged {
		return errors.New("activation was not acknowledged")
	}
	return nil
}

func (i *Instance) serve() {
	for {
		conn, err := i.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			var token string
			if json.NewDecoder(io.LimitReader(conn, 256)).Decode(&token) != nil || subtle.ConstantTimeCompare([]byte(token), []byte(i.endpoint.Token)) != 1 {
				return
			}
			select {
			case i.activations <- struct{}{}:
			default:
			}
			_ = json.NewEncoder(conn).Encode(true)
		}()
	}
}

func (i *Instance) Activations() <-chan struct{} { return i.activations }

func (i *Instance) Close() error {
	i.once.Do(func() {
		listenerErr := i.listener.Close()
		_ = os.Remove(i.path)
		i.closeErr = errors.Join(listenerErr, i.file.Close())
	})
	return i.closeErr
}
