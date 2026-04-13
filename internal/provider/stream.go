package provider

import (
	"io"
	"sync"

	"github.com/jrswab/axe/pkg/protocol"
)

type eventStream struct {
	nextFunc func() (protocol.StreamEvent, error)
	body     io.Closer
	closed   bool
	mu       sync.Mutex
}

func (s *eventStream) Next() (protocol.StreamEvent, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return protocol.StreamEvent{}, io.EOF
	}
	next := s.nextFunc
	s.mu.Unlock()
	return next()
}

func (s *eventStream) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	body := s.body
	s.mu.Unlock()
	return body.Close()
}

// NewEventStream returns a protocol.EventStream adapted from a nextFunc and closer.
func NewEventStream(body io.Closer, nextFunc func() (protocol.StreamEvent, error)) protocol.EventStream {
	return &eventStream{
		body:     body,
		nextFunc: nextFunc,
	}
}
