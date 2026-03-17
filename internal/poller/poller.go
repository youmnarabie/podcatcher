// internal/poller/poller.go
package poller

import (
	"context"
	"log"
	"time"

	"github.com/youmnarabie/poo/internal/ingester"
	"github.com/youmnarabie/poo/internal/store"
)

type Poller struct {
	store    *store.Store
	ingester *ingester.Ingester
	interval time.Duration
}

func New(s *store.Store, ing *ingester.Ingester, interval time.Duration) *Poller {
	return &Poller{store: s, ingester: ing, interval: interval}
}

func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollAll(ctx)
		}
	}
}

func (p *Poller) pollAll(ctx context.Context) {
	feeds, err := p.store.ListFeeds(ctx)
	if err != nil {
		log.Printf("poller: list feeds: %v", err)
		return
	}
	for _, f := range feeds {
		if err := p.ingester.FetchAndIngest(ctx, f.ID, f.URL); err != nil {
			log.Printf("poller: ingest %s: %v", f.URL, err)
		}
	}
}
