package main

import "context"

type claudeWebProbe struct {
	Connected        bool   `json:"connected"`
	Status           int    `json:"status,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
	Error            string `json:"error,omitempty"`
}

type claudeWebTransport interface {
	Available() error
	OpenLogin(context.Context) error
	Probe(context.Context) (claudeWebProbe, error)
	Complete(context.Context, string) (string, error)
	Close(context.Context) error
}
