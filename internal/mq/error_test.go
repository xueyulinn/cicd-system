package mq

import (
	"context"
	"errors"
	"fmt"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ErrClass
	}{
		{
			name: "context canceled",
			err:  context.Canceled,
			want: CtxDone,
		},
		{
			name: "wrapped connection closed",
			err:  fmt.Errorf("wrap: %w", ErrConnectionClosed),
			want: ConnLost,
		},
		{
			name: "wrapped fatal",
			err:  fmt.Errorf("wrap: %w", ErrFatal),
			want: Fatal,
		},
		{
			name: "amqp non-recoverable",
			err:  &amqp.Error{Code: 406, Reason: "precondition failed", Recover: false},
			want: Fatal,
		},
		{
			name: "default retryable",
			err:  errors.New("temporary unknown"),
			want: Retryable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyError(tt.err); got != tt.want {
				t.Fatalf("classifyError()=%v, want %v", got, tt.want)
			}
		})
	}
}
