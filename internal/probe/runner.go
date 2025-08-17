package probe

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	m "github.com/yoshitakumi/go-site-prober/pkg/metrics"
)

type Result struct {
	Target              string    `json:"target"`
	Status              int       `json:"status"`
	LatencyMillis       int64     `json:"latency_ms"`
	LastChecked         time.Time `json:"last_checked"`
	Up                  bool      `json:"up"`
	Error               string    `json:"error,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
}

type Runner struct {
	targets  []string
	interval time.Duration
	timeout  time.Duration

	mu      sync.RWMutex
	results map[string]Result

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

	up := (err == nil && status >= 200 && status < 400)

	r.mu.Lock()
	prev := r.results[target]
	res := Result{
		Target:        target,
		Status:        status,
		LatencyMillis: lat.Milliseconds(),
		LastChecked:   time.Now(),
		Up:            up,
	}
	if err != nil {
		res.Error = err.Error()
	} else if status < 200 || status >= 400 {
		res.Error = "non-success status"
	}

	if res.Up {
		res.ConsecutiveFailures = 0
	} else {
		if prev.Up {
			res.ConsecutiveFailures = 1
		} else {
			res.ConsecutiveFailures = prev.ConsecutiveFailures + 1
		}
	}
	r.results[target] = res
	r.mu.Unlock()

	codeLabel := "error"
	if err == nil {
		codeLabel = "success"
	} else if status != 0 {
		codeLabel = strconv.Itoa(status)
	}

	m.ProberDuration.With(prometheus.Labels{
		"target": target,
		"code":   codeLabel,
	}).Observe(lat.Seconds())

	if res.Up {
		m.ProberUp.With(prometheus.Labels{"target": target}).Set(1)
	} else {
		m.ProberUp.With(prometheus.Labels{"target": target}).Set(0)
	}
	m.ConsecutiveFailures.With(prometheus.Labels{"target": target}).Set(float64(res.ConsecutiveFailures))
}

func (r *Runner) Ready() bool {
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
