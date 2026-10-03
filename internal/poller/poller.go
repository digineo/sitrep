// Package poller polls the panels of all sites on their schedules and keeps
// the processed results in memory. It also keeps the change sequence that
// lets clients fetch only what changed since their last request.
package poller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/digineo/xlog"

	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/process"
	"github.com/digineo/sitrep/internal/store"
)

// Entry is the cached result of a panel.
type Entry struct {
	Data      process.Data
	FetchedAt time.Time // zero while pending: never polled successfully
	Stale     bool      // the latest poll failed
	Err       string    // the error of the latest poll, for admins
	Seq       uint64    // the change sequence of the latest visible change
}

// Pending reports whether the panel has not been polled successfully since
// the start or since its definition changed.
func (e Entry) Pending() bool {
	return e.FetchedAt.IsZero()
}

// Poller polls panels and caches their results.
type Poller struct {
	log            xlog.Logger
	db             *store.DB
	key            []byte
	defaultRefresh time.Duration
	boot           string

	reconciling sync.Mutex // serializes reconciliations
	wg          sync.WaitGroup

	mu      sync.Mutex
	stopped bool
	jobs    map[string]*job   // by panel ID
	entries map[string]*Entry // by panel ID
	seq     uint64
	sites   map[string]uint64 // sequence of each site's last change
	global  uint64            // sequence of the last instance settings change
}

// job polls one panel. A job is replaced whenever the panel's definition
// changes.
type job struct {
	key    jobKey
	cancel context.CancelFunc
}

// jobKey identifies a panel's definition: any change restarts the job.
type jobKey struct {
	panel, source int64
	refresh       time.Duration
}

// New returns a Poller. key decrypts data source secrets; panels without
// their own refresh are polled every defaultRefresh.
func New(
	log xlog.Logger,
	db *store.DB,
	key []byte,
	defaultRefresh time.Duration,
) *Poller {
	boot := make([]byte, 8)
	_, _ = rand.Read(boot)
	return &Poller{
		log:            log,
		db:             db,
		key:            key,
		defaultRefresh: defaultRefresh,
		boot:           hex.EncodeToString(boot),
		jobs:           map[string]*job{},
		entries:        map[string]*Entry{},
		sites:          map[string]uint64{},
	}
}

// Run reconciles the schedules with the database at once and then every
// minute. When ctx ends, it stops all schedules and waits for running polls.
func (p *Poller) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		p.Reconcile()
		select {
		case <-ctx.Done():
			p.stop()
			return
		case <-t.C:
		}
	}
}

func (p *Poller) stop() {
	p.mu.Lock()
	p.stopped = true
	for id, j := range p.jobs {
		j.cancel()
		delete(p.jobs, id)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// Reconcile starts, restarts and stops schedules to match the panels in
// the database. A panel whose definition or data source changed loses its
// cached result.
func (p *Poller) Reconcile() {
	p.reconciling.Lock()
	defer p.reconciling.Unlock()

	snap, err := p.db.Snapshot()
	if err != nil {
		p.log.Error("reading the panels to poll failed",
			xlog.Error(err))
		return
	}

	sources := map[string]model.DataSource{}
	for _, ds := range snap.DataSources {
		sources[ds.ID] = ds
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}

	polled := map[string]bool{}
	for _, panel := range snap.Panels {
		polled[panel.ID] = true
		ds := sources[panel.DataSource]
		refresh := model.Duration(panel.Refresh)
		if refresh == 0 {
			refresh = p.defaultRefresh
		}

		key := jobKey{
			panel:   panel.Revision,
			source:  ds.Revision,
			refresh: refresh,
		}
		if j := p.jobs[panel.ID]; j != nil {
			if j.key == key {
				continue
			}

			j.cancel()
		}

		ctx, cancel := context.WithCancel(context.Background())
		j := &job{
			key:    key,
			cancel: cancel,
		}
		p.jobs[panel.ID] = j
		p.entries[panel.ID] = &Entry{Seq: p.next()}
		p.wg.Go(func() { p.run(ctx, j, panel, ds) })
	}

	for id, j := range p.jobs {
		if !polled[id] {
			j.cancel()
			delete(p.jobs, id)
			delete(p.entries, id)
		}
	}
}

// run polls a panel at once and then every refresh, until ctx ends. Polls
// never overlap; ticks missed by a slow poll are skipped.
func (p *Poller) run(
	ctx context.Context,
	j *job,
	panel model.Panel,
	ds model.DataSource,
) {
	t := time.NewTicker(j.key.refresh)
	defer t.Stop()
	for {
		data, err := Evaluate(ctx, p.key, panel, ds, time.Now())
		p.store(j, panel, data, err)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

var errUnknownType = errors.New(
	"the data source type is not available in this build",
)

// Evaluate runs a panel's query against its data source and processes the
// result. key decrypts the data source's secrets.
func Evaluate(
	ctx context.Context,
	key []byte,
	panel model.Panel,
	ds model.DataSource,
	now time.Time,
) (process.Data, error) {
	t := datasource.Get(ds.Type)
	if t == nil {
		return process.Data{}, errUnknownType
	}

	cfg, err := datasource.Open(ds, key)
	if err != nil {
		return process.Data{}, err
	}

	res, err := t.Evaluate(ctx, cfg, panel, now)
	if err != nil {
		return process.Data{}, err
	}
	return process.Process(panel, res)
}

// store caches the outcome of a poll by j, unless j has been replaced or
// stopped meanwhile. A failure keeps the previous data and marks it stale.
func (p *Poller) store(j *job, panel model.Panel, data process.Data, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.jobs[panel.ID] != j {
		return
	}

	e := p.entries[panel.ID]
	if err == nil {
		p.entries[panel.ID] = &Entry{
			Data:      data,
			FetchedAt: time.Now(),
			Seq:       p.next(),
		}
		return
	}

	if e.Err != err.Error() {
		p.log.Warn("polling a panel failed",
			slog.String("site", panel.Site),
			slog.String("panel", panel.ID),
			xlog.Error(err))
	}

	e.Err = err.Error()
	if !e.Pending() && !e.Stale {
		e.Stale = true
		e.Seq = p.next()
	}
}

// Entry returns the cached result of a panel.
func (p *Poller) Entry(panel string) (Entry, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.entries[panel]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// next advances the change sequence. The caller holds p.mu.
func (p *Poller) next() uint64 {
	p.seq++
	return p.seq
}

// Seq returns the current change sequence number. Read it before the state
// it describes, so that later changes are never missed.
func (p *Poller) Seq() uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seq
}

// Cursor returns the opaque cursor for seq.
func (p *Poller) Cursor(seq uint64) string {
	return p.boot + "." + strconv.FormatUint(seq, 10)
}

// Since returns the sequence number of a cursor. It fails for malformed
// cursors and cursors of another process.
func (p *Poller) Since(cursor string) (uint64, bool) {
	boot, s, ok := strings.Cut(cursor, ".")
	seq, err := strconv.ParseUint(s, 10, 64)
	if !ok || boot != p.boot || err != nil || seq > p.Seq() {
		return 0, false
	}
	return seq, true
}

// SiteChanged records a change of a site or its panels, and reconciles the
// schedules.
func (p *Poller) SiteChanged(site string) {
	p.mu.Lock()
	p.sites[site] = p.next()
	p.mu.Unlock()
	p.Reconcile()
}

// SettingsChanged records a change of the instance settings, which sites
// inherit.
func (p *Poller) SettingsChanged() {
	p.mu.Lock()
	p.global = p.next()
	p.mu.Unlock()
}

// SiteSeq returns the sequence number of the last change of a site's
// settings or panel list.
func (p *Poller) SiteSeq(site string) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return max(p.sites[site], p.global)
}
