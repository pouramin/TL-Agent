//go:build !windows

package main

import (
	"context"
	"errors"
)

type claudeWebNativeTransport struct{}

func newClaudeWebNativeTransport() claudeWebTransport { return &claudeWebNativeTransport{} }

func (t *claudeWebNativeTransport) Available() error {
	return errors.New("Claude Web native Chrome bridge is currently available on Windows")
}

func (t *claudeWebNativeTransport) OpenLogin(context.Context) error { return t.Available() }
func (t *claudeWebNativeTransport) Probe(context.Context) (claudeWebProbe, error) {
	return claudeWebProbe{}, t.Available()
}
func (t *claudeWebNativeTransport) Complete(context.Context, string) (string, error) {
	return "", t.Available()
}
func (t *claudeWebNativeTransport) Close(context.Context) error { return nil }
