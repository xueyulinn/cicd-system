package orchestrator

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/xueyulinn/cicd-system/internal/mq"
)

const defaultPublisherConcurrent = 1

type publisherSet struct {
	gen     int
	conn    *amqp.Connection
	pubs    []mq.Publisher
	idx     atomic.Int64
	retired atomic.Bool
	refs    atomic.Int64
}

func (s *publisherSet) nextPublisher() mq.Publisher {
	if s == nil || len(s.pubs) == 0 {
		return nil
	}
	idx := s.idx.Add(1) - 1
	return s.pubs[idx%int64(len(s.pubs))]
}

func (s *publisherSet) Close() error {
	if s == nil {
		return nil
	}
	for _, publisher := range s.pubs {
		if publisher != nil {
			_ = publisher.Close()
		}
	}
	if s.conn != nil {
		_ = s.conn.Close()
	}
	return nil
}

type publisherManager struct {
	cfg         mq.Config
	mu          sync.RWMutex
	current     *publisherSet
	stale       map[int]*publisherSet
	concurrency int
}

func newPublisherManager(cfg mq.Config, concurrency int) (*publisherManager, error) {
	set, err := newPublisherSet(cfg, concurrency, 0)
	if err != nil {
		return nil, fmt.Errorf("create publisher set failed: %w", err)
	}

	return &publisherManager{
		cfg:         cfg,
		current:     set,
		stale:       make(map[int]*publisherSet),
		concurrency: concurrency,
	}, nil
}

func (m *publisherManager) releaseSet(set *publisherSet) {
	if set == nil {
		return
	}

	refs := set.refs.Add(-1)
	if refs == 0 && set.retired.Load() {
		m.mu.Lock()
		defer m.mu.Unlock()

		if set.retired.Load() && set.refs.Load() == 0 {
			delete(m.stale, set.gen)
			_ = set.Close()
		}
	}
}

func (m *publisherManager) acquireCurrentSet() *publisherSet {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.current == nil {
		return nil
	}
	m.current.refs.Add(1)
	return m.current
}

func (m *publisherManager) rebuildIfCurrent(failed *publisherSet) error {
	if failed == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.current != failed {
		return nil
	}

	newSet, err := newPublisherSet(m.cfg, m.concurrency, failed.gen+1)
	if err != nil {
		return fmt.Errorf("create publisher set failed: %w", err)
	}

	failed.retired.Store(true)
	m.stale[failed.gen] = failed
	m.current = newSet

	return nil
}

func (m *publisherManager) Close() error {
	if m == nil {
		return nil
	}

	var setsToClose []*publisherSet

	m.mu.Lock()
	if m.current != nil {
		m.current.retired.Store(true)
		m.stale[m.current.gen] = m.current
		m.current = nil
	}

	for gen, set := range m.stale {
		if set == nil {
			delete(m.stale, gen)
			continue
		}
		if set.refs.Load() == 0 {
			delete(m.stale, gen)
			setsToClose = append(setsToClose, set)
		}
	}
	m.mu.Unlock()

	for _, set := range setsToClose {
		if set != nil {
			_ = set.Close()
		}
	}

	return nil
}

func newPublisherSet(cfg mq.Config, concurrency int, gen int) (*publisherSet, error) {
	mqConn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("fail to connect RabbitMQ: %w", err)
	}

	publishers, err := createJobPublishers(cfg, mqConn, concurrency)
	if err != nil {
		_ = mqConn.Close()
		return nil, fmt.Errorf("fail to create publishers: %w", err)
	}

	return &publisherSet{
		gen:  gen,
		conn: mqConn,
		pubs: publishers,
	}, nil
}

func createJobPublishers(cfg mq.Config, conn *amqp.Connection, count int) ([]mq.Publisher, error) {
	if count < 1 {
		return nil, fmt.Errorf("publisher concurrency must be >= 1")
	}

	publishers := make([]mq.Publisher, 0, count)
	for i := 0; i < count; i++ {
		publisher, err := newJobPublisher(cfg, conn)
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

func newJobPublisher(cfg mq.Config, conn *amqp.Connection) (mq.Publisher, error) {
	mqClient, err := mq.NewRabbitClientWithConn(cfg, conn)
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
