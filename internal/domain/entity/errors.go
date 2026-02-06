package entity

import "errors"

// Standard domain errors
var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded: too many tokens used")
	ErrInternalServer    = errors.New("an internal error occurred")
	ErrInvalidRequest    = errors.New("invalid request parameters")
	ErrResourceNotFound  = errors.New("the requested resource was not found")
)
