package lock

import (
	"context"
	"time"
)

type Locker interface {
	TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error)
}

type Lock interface {
	Release(ctx context.Context) error
}

type NoopLocker struct{}

func (NoopLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (Lock, bool, error) {
	return noopLock{}, true, nil
}

type noopLock struct{}

func (noopLock) Release(ctx context.Context) error {
	return nil
}
