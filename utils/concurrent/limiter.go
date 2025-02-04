package concurrent

import "sync"

type Limiter struct {
    ch chan struct{}
    wg sync.WaitGroup
}

func NewLimiter(maxConcurrent int) *Limiter {
    return &Limiter{
        ch: make(chan struct{}, maxConcurrent),
    }
}

func (l *Limiter) Execute(fn func()) {
    l.wg.Add(1)
    l.ch <- struct{}{}
    
    go func() {
        defer func() {
            <-l.ch
            l.wg.Done()
        }()
        fn()
    }()
}

func (l *Limiter) Wait() {
    l.wg.Wait()
} 