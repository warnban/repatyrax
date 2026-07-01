package service

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/tyrax/tyrax-backend/internal/model"
)

// Load-balancing sampler tuning.
const (
	// balancerInterval is how often live per-node load is refreshed from panels.
	balancerInterval = 30 * time.Second
	// balancerSampleTTL is how long a sample stays "fresh". Past this the node's
	// load is treated as unknown, disabling balancing (fail-open to ping order).
	balancerSampleTTL = 100 * time.Second
	// balancerTimeout bounds a single sampling pass.
	balancerTimeout = 25 * time.Second
)

// OnlinesReader reports how many clients are currently online on a node's panel.
// Implemented by pkg/threexui.Syncer.
type OnlinesReader interface {
	Onlines(ctx context.Context, node model.Node) (int, error)
}

type loadSample struct {
	count int
	at    time.Time
}

// NodeBalancer keeps a short-lived, in-memory view of each node's live client
// count and exposes it for load-aware node ordering. Fully fail-open: if a node
// has never been sampled (or its sample is stale) its load is "unknown", and the
// caller falls back to the default (ping) ordering.
type NodeBalancer struct {
	nodeRepo interface {
		List(ctx context.Context) ([]model.Node, error)
	}
	reader OnlinesReader

	mu      sync.RWMutex
	samples map[string]loadSample // nodeID -> last load sample
}

func NewNodeBalancer(nodeRepo interface {
	List(ctx context.Context) ([]model.Node, error)
}, reader OnlinesReader) *NodeBalancer {
	return &NodeBalancer{
		nodeRepo: nodeRepo,
		reader:   reader,
		samples:  make(map[string]loadSample),
	}
}

// NodeLoad returns the last known online-client count for a node and whether it
// is fresh enough to trust. fresh == false means "unknown" — do not balance on it.
func (b *NodeBalancer) NodeLoad(nodeID string) (count int, fresh bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	s, ok := b.samples[nodeID]
	if !ok || time.Since(s.at) > balancerSampleTTL {
		return 0, false
	}
	return s.count, true
}

// RunLoop refreshes load samples on a ticker until ctx is cancelled.
func (b *NodeBalancer) RunLoop(ctx context.Context) {
	ticker := time.NewTicker(balancerInterval)
	defer ticker.Stop()

	b.sampleSafe(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.sampleSafe(ctx)
		}
	}
}

func (b *NodeBalancer) sampleSafe(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, balancerTimeout)
	defer cancel()

	nodes, err := b.nodeRepo.List(ctx)
	if err != nil {
		slog.Warn("balancer: list nodes", "err", err.Error())
		return
	}
	for _, n := range nodes {
		// Only meterable nodes: OPEN vless with a panel. Others stay "unknown".
		if n.Status != model.NodeOpen || n.Protocol != "vless" || n.PanelURL == "" {
			continue
		}
		count, err := b.reader.Onlines(ctx, n)
		if err != nil {
			// Leave the previous sample to go stale on its own; log at debug so a
			// fork without the onlines endpoint doesn't spam the log.
			slog.Debug("balancer: onlines", "node", n.Codename, "err", err.Error())
			continue
		}
		b.mu.Lock()
		b.samples[n.ID] = loadSample{count: count, at: time.Now()}
		b.mu.Unlock()
	}
}
