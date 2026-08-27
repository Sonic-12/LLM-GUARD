package middleware

import (
	"context"
	"fmt"
)

type RequestContext struct {
	RequestID string
	UserID    string
	Metadata  map[string]any
}

type PreHook interface {
	Name() string
	HandleRequest(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error)
}

type PostHook interface {
	Name() string
	HandleResponse(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error)
}

type Chain struct {
	pre  []PreHook
	post []PostHook
}

func NewChain() *Chain {
	return &Chain{}
}

func (c *Chain) UsePre(h PreHook) {
	c.pre = append(c.pre, h)
}

func (c *Chain) UsePost(h PostHook) {
	c.post = append(c.post, h)
}

func (c *Chain) RunPre(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error) {
	var err error
	for _, h := range c.pre {
		body, err = h.HandleRequest(ctx, rc, body)
		if err != nil {
			return nil, fmt.Errorf("middleware[%s]: %w", h.Name(), err)
		}
	}
	return body, nil
}
func (c *Chain) RunPost(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error) {
	var err error
	for _, h := range c.post {
		body, err = h.HandleResponse(ctx, rc, body)
		if err != nil {
			return nil, fmt.Errorf("middleware[%s]: %w", h.Name(), err)
		}
	}
	return body, nil
}
