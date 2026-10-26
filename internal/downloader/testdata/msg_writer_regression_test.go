package torrent

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/anacrolix/chansync"
	"github.com/anacrolix/log"

	pp "github.com/anacrolix/torrent/peer_protocol"
)

// These tests run inside an isolated copy of the pinned torrent module so they
// exercise its private writer without copying the implementation into Dipper.
func startDipperRegressionWriter(w *peerConnMsgWriter, interval time.Duration) func() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(interval)
	}()
	return func() {
		w.closed.Set()
		<-done
	}
}

func TestDipperWriterRetainsWakeupDuringFill(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var fills atomic.Int32
		w := &peerConnMsgWriter{
			closed:      new(chansync.SetOnce),
			logger:      log.Default,
			w:           io.Discard,
			keepAlive:   func() bool { return false },
			writeBuffer: new(peerConnMsgWriterBuffer),
		}
		w.fillWriteBuffer = func() {
			if fills.Add(1) == 1 {
				w.writeCond.Broadcast()
			}
		}
		stop := startDipperRegressionWriter(w, time.Hour)
		defer stop()
		synctest.Wait()
		if got := fills.Load(); got != 2 {
			t.Fatalf("writer lost the wakeup during fill: filled %d times, want 2", got)
		}
	})
}

func TestDipperWriterRearmsIdleKeepalive(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const interval = time.Minute
		var useful atomic.Bool
		var output bytes.Buffer
		w := &peerConnMsgWriter{
			fillWriteBuffer: func() {},
			closed:          new(chansync.SetOnce),
			logger:          log.Default,
			w:               &output,
			keepAlive:       func() bool { return useful.CompareAndSwap(true, false) },
			writeBuffer:     new(peerConnMsgWriterBuffer),
		}
		stop := startDipperRegressionWriter(w, interval)
		defer stop()
		synctest.Wait()
		// Expire once with no output, then make the peer useful without a tickle.
		time.Sleep(interval)
		synctest.Wait()
		useful.Store(true)
		time.Sleep(interval)
		synctest.Wait()
		want := (pp.Message{Keepalive: true}).MustMarshalBinary()
		if !bytes.Equal(output.Bytes(), want) {
			t.Fatalf("idle writer did not rearm keepalive: got %x, want %x", output.Bytes(), want)
		}
	})
}
