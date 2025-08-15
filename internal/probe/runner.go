package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Result struct {
	Target        string
	Status        int
	LatencyMillis int64
	LastChecked   time.Time
	Up            bool

	Error string
}

type Runner struct {
	targets        []string
	interval       time.Duration
	timeout        time.Duration
	mu             sync.RWMutex
	results        map[string]Result
	firstRoundDone atomic.Bool
}

func NewRunner(targets []string, interval, timeout time.Duration) *Runner {
	return &Runner{
		targets:  targets,
		interval: interval,
		timeout:  timeout,
		results:  make(map[string]Result),
	}
}

func (r *Runner) Start(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(len(r.targets))

	for _, t := range r.targets {
		target := t
		go func() {
			defer wg.Done()
			r.runOnce(ctx, target)
			ticker := time.NewTicker(r.interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-ticker.C:
					r.runOnce(ctx, target)
				}

			}
		}()
	}

	go func() {
		wg.Wait()
		r.firstRoundDone.Store(true)
	}()
}

func (r *Runner) runOnce(ctx context.Context, target string) {

	pctx, cancel := context.WithTimeout(ctx, r.timeout)

	defer cancel()

	client := &http.Client{Timeout: r.timeout}
	start := time.Now()
	resp, err := client.Get(target)
	var status int

	if err == nil && resp != nil {
		status = resp.StatusCode
		_ = resp.Body.Close()
	}

	lat := time.Since(start)

	res := Result{
		Target:        target,
		Status:        status,
		LatencyMillis: lat.Milliseconds(),
		LastChecked:   time.Now(),
		Up:            err == nil && status >= 200 && status < 400,
	}

	if err != nil {
		res.Error = err.Error()
	} else if status < 200 || status >= 400 {
		res.Error = "non-success"
	}

	r.mu.Lock()
	r.results[target] = res
	r.mu.Unlock()

}

func (r *Runner) Ready() bool {
	// ready after first round
	return r.firstRoundDone.Load()
}

func (r *Runner) ResultsJSON() []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	arr := make([]Result, 0, len(r.results))
	for _, v := range r.results {
		arr = append(arr, v)
	}
	b, _ := json.MarshalIndent(arr, "", "  ")
	return b
}
