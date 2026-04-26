package mq

import (
	"context"
	"errors"
	"net"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ErrClass classifies MQ errors for retry/exit decisions.
type ErrClass int

const (
	Retryable ErrClass = iota
	Fatal
	CtxDone
	ConnLost
)

var (
	// ErrConnectionClosed indicates that the AMQP connection is no longer usable.
	ErrConnectionClosed = errors.New("rabbit connection closed")
	// ErrFatal marks unrecoverable MQ client errors (for example invalid configuration).
	ErrFatal = errors.New("rabbit fatal error")
)

func classifyError(err error) ErrClass {
	if err == nil {
		return Retryable
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return CtxDone
	}
	if errors.Is(err, ErrConnectionClosed) {
		return ConnLost
	}
	if errors.Is(err, ErrFatal) {
		return Fatal
	}

	var amqpErr *amqp.Error
	if errors.As(err, &amqpErr) && !amqpErr.Recover {
		return Fatal
	}

	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout()) {
		return Retryable
	}

	return Retryable
}
