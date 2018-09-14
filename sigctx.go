package sigctx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
)

var SignalReceived error = errors.New("signal received")

// closedchan is a reusable closed channel.
var closedchan = make(chan struct{})

func init() {
	close(closedchan)
}

type signalCtx struct {
	context.Context

	sig       []os.Signal
	ch        chan os.Signal
	ctxCancel context.CancelFunc
	mu        sync.Mutex
	done      chan struct{}
	err       error
}

func (c *signalCtx) Done() <-chan struct{} {
	c.mu.Lock()
	if c.done == nil {
		c.done = make(chan struct{})
	}
	d := c.done
	c.mu.Unlock()
	return d
}

func (c *signalCtx) Err() error {
	c.mu.Lock()
	err := c.err
	c.mu.Unlock()
	return err
}

func (c *signalCtx) String() string {
	return fmt.Sprintf("%v.WithCancelBySignal", c.Context)
}

func (c *signalCtx) cancel(err error) {
	if err == nil {
		panic("sigctx: internal error: missing cancel error")
	}

	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return // already canceled
	}

	c.err = err
	if c.done == nil {
		c.done = closedchan
	} else {
		close(c.done)
	}

	signal.Reset(c.sig...)
	close(c.ch)

	c.mu.Unlock()
}

func (c *signalCtx) watch() {
	select {
	case <-c.Context.Done():
		c.cancel(c.Context.Err())
	case <-c.ch:
		c.ctxCancel()
		c.cancel(SignalReceived)
	}
}

func WithCancelBySignal(parent context.Context, sig ...os.Signal) (ctx context.Context, cancel context.CancelFunc) {
	wrap, cancel := context.WithCancel(parent)

	c := &signalCtx{
		Context:   wrap,
		sig:       sig,
		ch:        make(chan os.Signal),
		ctxCancel: cancel,
	}

	signal.Notify(c.ch, sig...)
	go c.watch()

	return c, cancel
}
