//go:build !unix

package persistence

import (
	"fmt"
	"os"
	"time"
)

// lockStaleAfter bounds how long a leftover lock file from a crashed process is
// honored before being reclaimed, so the data directory cannot become
// permanently unusable after a crash on platforms without advisory file locks.
const lockStaleAfter = 5 * time.Second

// acquireFileLock obtains an exclusive, cross-process lock on path by
// atomically creating a lock file. It retries briefly while another writer
// holds the lock and reclaims stale locks left by a crashed holder.
func acquireFileLock(path string) (func(), error) {
	for attempt := 0; ; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			return func() { _ = f.Close(); _ = os.Remove(path) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > lockStaleAfter {
			_ = os.Remove(path)
			continue
		}
		if attempt >= lockBusyRetries {
			return nil, fmt.Errorf("数据目录正被其他进程写入，请稍后重试")
		}
		time.Sleep(lockBusySleep)
	}
}
