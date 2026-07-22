package internal

import "context"

type WinbindThrottler struct {
	sem chan struct{}
}

func NewWinbindThrottler(maxConcurrent int) *WinbindThrottler {
	return &WinbindThrottler{
		sem: make(chan struct{}, maxConcurrent),
	}
}

func (t *WinbindThrottler) Acquire(ctx context.Context) error {
	select {
	case t.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (t *WinbindThrottler) Release() {
	<-t.sem
}
