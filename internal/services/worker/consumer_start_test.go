package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xueyulinn/cicd-system/internal/messages"
	"github.com/xueyulinn/cicd-system/internal/mq"
	"github.com/moby/moby/client"
)

type startFakeConsumer struct {
	consumeFn func(context.Context, func(context.Context, messages.JobExecutionMessage) error) error
}

func (f *startFakeConsumer) ConsumeJob(ctx context.Context, handler func(context.Context, messages.JobExecutionMessage) error) error {
	if f.consumeFn != nil {
		return f.consumeFn(ctx, handler)
	}
	return nil
}

func (f *startFakeConsumer) Close() error { return nil }

func TestStart_NilService(t *testing.T) {
	var svc *Service
	err := svc.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "worker service is nil") {
		t.Fatalf("err=%v", err)
	}
}

func TestStart_RequiresDockerClient(t *testing.T) {
	svc := &Service{jobConsumers: []mq.Consumer{&startFakeConsumer{}}}
	err := svc.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "docker client not available") {
		t.Fatalf("err=%v", err)
	}
}

func TestStart_RequiresJobConsumer(t *testing.T) {
	svc := &Service{docker: &client.Client{}}
	err := svc.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "job consumer not available") {
		t.Fatalf("err=%v", err)
	}
}

func TestStart_ReturnsConsumerError(t *testing.T) {
	wantErr := errors.New("consume failed")
	svc := &Service{docker: &client.Client{}, jobConsumers: []mq.Consumer{
		&startFakeConsumer{consumeFn: func(context.Context, func(context.Context, messages.JobExecutionMessage) error) error { return wantErr }},
		&startFakeConsumer{consumeFn: func(ctx context.Context, _ func(context.Context, messages.JobExecutionMessage) error) error {
			<-ctx.Done()
			return nil
		}},
	}}

	err := svc.Start(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "job consumer 1 failed") || !strings.Contains(err.Error(), "consume failed") {
		t.Fatalf("err=%v", err)
	}
}

func TestStart_ReturnsContextCancellation(t *testing.T) {
	svc := &Service{docker: &client.Client{}, jobConsumers: []mq.Consumer{
		&startFakeConsumer{consumeFn: func(ctx context.Context, _ func(context.Context, messages.JobExecutionMessage) error) error {
			<-ctx.Done()
			return nil
		}},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.Start(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestStart_ReturnsNilWhenConsumersExitCleanly(t *testing.T) {
	svc := &Service{docker: &client.Client{}, jobConsumers: []mq.Consumer{
		&startFakeConsumer{consumeFn: func(ctx context.Context, _ func(context.Context, messages.JobExecutionMessage) error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Millisecond):
				return nil
			}
		}},
	}}
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("err=%v", err)
	}
}
