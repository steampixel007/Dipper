package downloader

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	connectivityHint = syscall.NewLazyDLL("iphlpapi.dll").NewProc("GetNetworkConnectivityHint")
	unbiasedTime     = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryUnbiasedInterruptTime")
)

func networkAvailable() bool {
	if connectivityHint.Find() != nil {
		return false
	}
	var hint struct {
		Level, Cost                     uint32
		Approaching, Over, Roaming, Pad byte
	}
	code, _, _ := connectivityHint.Call(uintptr(unsafe.Pointer(&hint)))
	return code == 0 && hint.Level == 3
}

func awakeTime() time.Duration {
	var ticks uint64
	ok, _, _ := unbiasedTime.Call(uintptr(unsafe.Pointer(&ticks)))
	if ok == 0 {
		return 0
	}
	return time.Duration(ticks) * 100 * time.Nanosecond
}
