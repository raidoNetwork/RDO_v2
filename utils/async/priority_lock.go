package async

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	free int32 = iota
	locked
)

type Mutex struct {
	state int32
	mu sync.Mutex
}

func (m *Mutex) Lock() {
	m.mu.Lock()
	atomic.CompareAndSwapInt32(&m.state, free, locked)
}

func (m *Mutex) WaitLock() {
	for {
		if atomic.LoadInt32(&m.state) == free {
			return
		}

		<-time.After(20 * time.Millisecond)
	}
}

func (m *Mutex) Unlock() {
	m.mu.Unlock()
	atomic.CompareAndSwapInt32(&m.state, locked, free)
}
