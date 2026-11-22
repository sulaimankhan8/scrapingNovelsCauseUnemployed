//go:build !windows

package browser

import (
	"context"

	"novelarchiver/internal/fetch"
)

// Window is unavailable on this platform.
type Window struct{}

// New always fails on non-Windows platforms for now.
func New(Options) (*Window, error) { return nil, ErrUnsupported }

// Get implements fetch.Fetcher.
func (*Window) Get(context.Context, string, string) (*fetch.Page, error) { return nil, ErrUnsupported }

// Show is a no-op.
func (*Window) Show(string) error { return ErrUnsupported }

// Close is a no-op.
func (*Window) Close() {}
