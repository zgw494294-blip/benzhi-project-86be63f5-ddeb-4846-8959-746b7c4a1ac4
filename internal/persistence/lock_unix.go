//go:build unix

package persistence

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// acquireFileLock obtains an exclusive, cross-process lock on path using an
// advisory file lock (flock). The lock is released automatically by the kernel
// when the holding file descriptor is closed or the owning process exits, so a
// crash never leaves a stale lock that blocks the data directory. It returns a
// release function the caller must invoke when done.
//
// The advisory lock is combined with the in-process mutex held by withLock, so
// access is serialized both across goroutines and across processes sharing the
// same data directory.
func acquireFileLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return func() {
				_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				_ = f.Close()
			}, nil
		}
		if attempt >= lockBusyRetries {
			_ = f.Close()
			return nil, fmt.Errorf("数据目录正被其他进程写入，请稍后重试")
		}
		time.Sleep(lockBusySleep)
	}
}
