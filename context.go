package wander

import "context"

type PausableContext struct {
	context.Context
	pause  chan struct{}
	resume chan struct{}
}

func NewPausableContext(ctx context.Context) (*PausableContext, func(bool)) {
	pausable := &PausableContext{
		ctx,
		make(chan struct{}),
		make(chan struct{}),
	}

	return pausable, func(pause bool) {
		if pause {
			pausable.pause <- struct{}{}
		} else {
			pausable.resume <- struct{}{}
		}

	}
}

func (p *PausableContext) Pause() <-chan struct{} {
	return p.pause
}

func (p *PausableContext) Resume() <-chan struct{} {
	return p.resume
}
