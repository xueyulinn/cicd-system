package execution

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
)

const defaultPublisherConcurrent = 1

var newJobPublisher = func(cfg mq.Config) (mq.Publisher, error) {
	mqClient, err := mq.NewRabbitClient(cfg)
	if err != nil {
		return nil, err
	}

	jobPublisher, err := mq.NewJobPublisher(mqClient, cfg)
	if err != nil {
		_ = mqClient.Close()
		return nil, err
	}
	return jobPublisher, nil
}

func createJobPublishers(cfg mq.Config, count int) ([]mq.Publisher, error) {
	if count < 1 {
		return nil, fmt.Errorf("publisher concurrency must be >= 1")
	}

	publishers := make([]mq.Publisher, 0, count)
	for i := 0; i < count; i++ {
		publisher, err := newJobPublisher(cfg)
		if err != nil {
			for _, c := range publishers {
				_ = c.Close()
			}
			return nil, fmt.Errorf("initialize job publisher %d/%d: %w", i+1, count, err)
		}
		publishers = append(publishers, publisher)
	}
	return publishers, nil
}


func loadPublisherConcurrency() int {
	raw := strings.TrimSpace(os.Getenv("PUBLISHER_CONCURRENCY"))
	if raw == "" {
		return defaultPublisherConcurrent
	}

	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		log.Printf("[publisher] invalid PUBLISHER_CONCURRENCY=%q, fallback=%d", raw, defaultPublisherConcurrent)
		return defaultPublisherConcurrent
	}
	return v
}
