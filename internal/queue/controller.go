package queue

import (
	"context"
	"time"

	"github.com/webmalex/ch_watch/internal/model"
	"github.com/webmalex/ch_watch/internal/report"
	"github.com/webmalex/ch_watch/internal/runner"
)

type SnapshotFunc func(path string, now time.Time) (model.FileFingerprint, bool, error)

type ControllerConfig struct {
	Debounce time.Duration
	Suppress time.Duration
	Now      func() time.Time
	Request  model.RunRequest
}

type Controller struct {
	config     ControllerConfig
	snapshot   SnapshotFunc
	runner     runner.Runner
	reporter   report.Reporter
	notifyCh   chan string
	runDoneCh  chan model.RunResult
	waitIdleCh chan chan struct{}
}

func NewController(cfg ControllerConfig, snapshot SnapshotFunc, run runner.Runner, rep report.Reporter) *Controller {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 75 * time.Millisecond
	}
	return &Controller{
		config:     cfg,
		snapshot:   snapshot,
		runner:     run,
		reporter:   rep,
		notifyCh:   make(chan string, 128),
		runDoneCh:  make(chan model.RunResult, 16),
		waitIdleCh: make(chan chan struct{}),
	}
}

func (c *Controller) Notify(path string) {
	c.notifyCh <- path
}

func (c *Controller) WaitIdle(ctx context.Context) error {
	ready := make(chan struct{})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.waitIdleCh <- ready:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ready:
		return nil
	}
}

func (c *Controller) Run(ctx context.Context) error {
	batch := NewOrderedSet()
	pending := NewOrderedSet()
	suppressor := NewSuppressor(c.config.Suppress, c.config.Now)
	var idleWaiters []chan struct{}
	var running bool
	var timer *time.Timer
	var timerCh <-chan time.Time

	startNext := func() {
		if running {
			return
		}
		next, ok := pending.PopFront()
		if !ok {
			return
		}
		running = true
		c.reporter.Run(next)
		go func(path string) {
			request := c.config.Request
			request.Path = path
			result := c.runner.Run(ctx, request)
			c.runDoneCh <- result
		}(next)
	}

	flushIdleWaiters := func() {
		if running || batch.Len() > 0 || pending.Len() > 0 || timerCh != nil {
			return
		}
		for _, waiter := range idleWaiters {
			close(waiter)
		}
		idleWaiters = nil
	}

	flushBatch := func() {
		timerCh = nil
		for _, path := range batch.Drain() {
			fingerprint, ok, err := c.snapshot(path, c.config.Now())
			if err != nil || !ok {
				continue
			}
			if !suppressor.Allow(fingerprint) {
				continue
			}
			pending.Add(fingerprint.Path)
		}
		startNext()
		flushIdleWaiters()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case waiter := <-c.waitIdleCh:
			idleWaiters = append(idleWaiters, waiter)
			flushIdleWaiters()
		case path := <-c.notifyCh:
			batch.Add(path)
			if timer == nil {
				timer = time.NewTimer(c.config.Debounce)
				timerCh = timer.C
				continue
			}
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(c.config.Debounce)
			timerCh = timer.C
		case <-timerCh:
			flushBatch()
		case result := <-c.runDoneCh:
			running = false
			c.reporter.Result(result)
			startNext()
			flushIdleWaiters()
		}
	}
}
