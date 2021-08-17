package main

import (
	"context"
	"sync"
	"time"
)

/* global variable declaration, if any... */
var isLocked sync.Map

func isLockedAndMakeLock(ctx context.Context, hash string, count int) bool {

	id := getReqIDFromContext(ctx)

	defer isLocked.Delete(hash)

	value, alreadyLoaded := isLocked.LoadOrStore(hash, true)

	if !alreadyLoaded {
		info.Printf("%sissuing temporary lock for %s\n", id, hash)
		return false
	}

	if _, ok := value.(bool); !ok {
		fail.Printf("%sreturned value is not of bool type\n", id)
		return false
	}

	if !value.(bool) {
		info.Printf("%sno lock for %s, continuing...\n", id, hash)
		return false
	}

	info.Printf("%stemporary lock in effect for %s, waiting...\n", id, hash)

	for i := 1; i <= count; i++ {

		time.Sleep(1 * time.Second)

		info.Printf("%schecking for temporary lock %d/%d on %s\n", id, i, count, hash)

		value, ok := isLocked.Load(hash)

		if !ok {
			info.Printf("%sno lock for %s, continuing...\n", id, hash)
			return false
		}

		if _, ok := value.(bool); !ok {
			fail.Printf("%sreturned value is not of bool type\n", id)
			continue
		}

		if !value.(bool) {
			info.Printf("%sno lock for %s, continuing...\n", id, hash)
			return false
		}

	}

	info.Printf("%slock in effect for %s, try again later\n", id, hash)
	return true
}

func removeLock(ctx context.Context, hash string) bool {

	id := getReqIDFromContext(ctx)

	info.Printf("%sremoving temporary lock for %s\n", id, hash)
	isLocked.Delete(hash)

	time.Sleep(1 * time.Second)

	return true
}

func addLock(ctx context.Context, hash string) bool {

	id := getReqIDFromContext(ctx)

	info.Printf("%sadding temporary lock for %s\n", id, hash)
	isLocked.Store(hash, true)

	time.Sleep(1 * time.Second)

	return true
}

func listLocks(ctx context.Context) []string {

	id := getReqIDFromContext(ctx)

	locks := make([]string, 0)

	s := make(map[interface{}]interface{})
	isLocked.Range(func(k, v interface{}) bool {
		s[k] = v
		return true
	})

	for k, v := range s {
		status, ok := v.(bool)
		if ok && status {
			lock, ok := k.(string)
			if ok {
				locks = append(locks, lock)
			} else {
				fail.Printf("%sreturned value is not of string type\n", id)
			}
		} else if !ok {
			fail.Printf("%sreturned value is not of bool type\n", id)
		}
	}

	info.Printf("%sacquired list of mutex locks\n", id)

	return locks
}
