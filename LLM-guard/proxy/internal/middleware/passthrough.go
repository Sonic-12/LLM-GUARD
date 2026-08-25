package middleware

import "context"

// Passthrough is a no-op hook; it's for future hooks and proof the chain executes.
type Passthrough struct{}

func (Passthrough) Name() string { return "passthrough" }

func (Passthrough) HandleRequest(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error) {
	return body, nil
}

func (Passthrough) HandleResponse(ctx context.Context, rc *RequestContext, body []byte) ([]byte, error) {
	return body, nil
}