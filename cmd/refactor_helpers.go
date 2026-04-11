package cmd

import (
	"io"

	"github.com/jrswab/axe/internal/provider"
	"github.com/jrswab/axe/pkg/kernel"
)

const maxToolOutputBytes = 1024

func truncateOutput(s string) string {
	k := &kernel.Kernel{}
	return k.TruncateOutput(s)
}

func drainEventStream(stream provider.EventStream, w io.Writer) (*provider.Response, error) {
	k := &kernel.Kernel{}
	return k.DrainEventStream(stream, w)
}
