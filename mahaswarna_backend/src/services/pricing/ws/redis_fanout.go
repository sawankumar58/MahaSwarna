package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	contractsevents "github.com/mahaswarna/contracts/events"
)

const (
	// fanoutBatchWindow is the maximum time to wait before flushing the latest rate
	// to WebSocket clients. Latest-only fanout: only the most recent update per
	// cityID within the window is sent. Intermediate updates are intentionally dropped —
	// gold rates are sampled observations, not a financial ledger.
	fanoutBatchWindow = 50 * time.Millisecond
)

// BufferedFanout batches incoming rate updates and pushes only the latest per city
// to subscribed WebSocket clients within each 50ms window.
type BufferedFanout struct {
	registry   *ConnectionRegistry
	citySubsMu sync.RWMutex
	citySubs   map[string][]string // cityID → []userID
	pendingMu  sync.Mutex
	pending    map[string]*contractsevents.AIRateSnapshotReadyPayload // cityID → latest update
}

func NewBufferedFanout(registry *ConnectionRegistry) *BufferedFanout {
	return &BufferedFanout{
		registry: registry,
		citySubs: make(map[string][]string),
		pending:  make(map[string]*contractsevents.AIRateSnapshotReadyPayload),
	}
}

// Subscribe registers userID to receive rate updates for cityID.
func (f *BufferedFanout) Subscribe(cityID, userID string) {
	f.citySubsMu.Lock()
	defer f.citySubsMu.Unlock()
	f.citySubs[cityID] = append(f.citySubs[cityID], userID)
}

// Unsubscribe removes a user from a city's subscriber list.
func (f *BufferedFanout) Unsubscribe(cityID, userID string) {
	f.citySubsMu.Lock()
	defer f.citySubsMu.Unlock()
	subs := f.citySubs[cityID]
	updated := subs[:0]
	for _, id := range subs {
		if id != userID {
			updated = append(updated, id)
		}
	}
	f.citySubs[cityID] = updated
}

// HandleNotification is registered with pgnotify.Listener for ai_rate_snapshot_ready.
// It buffers the full payload; the flush loop delivers it with the correct source field.
func (f *BufferedFanout) HandleNotification(_ context.Context, _, payload string) {
	var p contractsevents.AIRateSnapshotReadyPayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		slog.Warn("fanout: malformed ai_rate_snapshot_ready payload", "err", err)
		return
	}

	f.pendingMu.Lock()
	// Latest-only: overwrite any pending update for this city.
	f.pending[p.CityID] = &p
	f.pendingMu.Unlock()
}

// Run starts the flush loop. Call in a goroutine: go fanout.Run(ctx).
func (f *BufferedFanout) Run(ctx context.Context) {
	ticker := time.NewTicker(fanoutBatchWindow)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f.flush()
		}
	}
}

func (f *BufferedFanout) flush() {
	f.pendingMu.Lock()
	if len(f.pending) == 0 {
		f.pendingMu.Unlock()
		return
	}
	// Swap the pending map so we release the lock quickly.
	batch := f.pending
	f.pending = make(map[string]*contractsevents.AIRateSnapshotReadyPayload)
	f.pendingMu.Unlock()

	f.citySubsMu.RLock()
	defer f.citySubsMu.RUnlock()

	for cityID, p := range batch {
		subs := f.citySubs[cityID]
		if len(subs) == 0 {
			continue
		}

		// Derive source from the snapshot payload. The notifier sets Source from
		// domain.SourceGemini, domain.SourceManualOverride, etc. Stale overrides
		// source to "stale" so the Android StaleRateBanner renders regardless of
		// the underlying source.
		//
		// OpenAPI invariant: never hardcode "gemini" as a string literal —
		// always derive from rate.source.
		source := p.Source
		if p.Stale {
			source = "stale"
		}

		msg := OutboundRateMessage{
			Channel: ChannelRates,
			CityID:  p.CityID,
			Gold:    p.Gold,
			Silver:  p.Silver,
			Stale:   p.Stale,
			Source:  source,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			slog.Warn("fanout marshal error", "city", cityID, "err", err)
			continue
		}

		for _, userID := range subs {
			f.registry.Send(userID, b)
		}
	}
}
