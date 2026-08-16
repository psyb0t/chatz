package upstreams

import "errors"

var (
	ErrInvalidLimits     = errors.New("invalid upstream runtime limits")
	ErrFirstTokenTimeout = errors.New("upstream time-to-first-token exceeded")
	ErrTurnTimeout       = errors.New("upstream total-turn timeout exceeded")
	ErrDiscoveryTimeout  = errors.New(
		"upstream model discovery timeout exceeded",
	)
	ErrInvalidDiscovery = errors.New(
		"invalid upstream discovery configuration",
	)
	ErrNilDriver           = errors.New("upstream driver is nil")
	ErrNilHealthTracker    = errors.New("upstream health tracker is nil")
	ErrInvalidUpstreamName = errors.New("upstream name is empty")
)
