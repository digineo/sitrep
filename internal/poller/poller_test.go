package poller

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/digineo/xlog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
	"github.com/digineo/sitrep/internal/datasource"
	"github.com/digineo/sitrep/internal/model"
	"github.com/digineo/sitrep/internal/process"
	"github.com/digineo/sitrep/internal/store"
)

// fake answers with its current value or error. While blocked, evaluations
// wait until released or cancelled.
type fake struct {
	mu      sync.Mutex
	value   float64
	err     error
	calls   int
	block   chan struct{}
	started chan struct{}
}

func (*fake) Fields() []datasource.Field {
	return []datasource.Field{{
		Name: "token",
		Kind: datasource.KindSecret,
	}}
}

func (*fake) PanelTypes() []string { return []string{model.PanelStat} }

func (*fake) Test(context.Context, datasource.Config) (string, error) {
	return "", nil
}

func (*fake) Summary(datasource.Config) string { return "" }

func (f *fake) Evaluate(
	ctx context.Context,
	_ datasource.Config,
	_ model.Panel,
	_ time.Time,
) (datasource.Result, error) {
	f.mu.Lock()
	f.calls++
	v, err := f.value, f.err
	block, started := f.block, f.started
	f.mu.Unlock()

	if block != nil {
		started <- struct{}{}
		select {
		case <-block:
		case <-ctx.Done():
			return datasource.Result{}, ctx.Err()
		}
	}
	return datasource.Result{Scalar: &v}, err
}

func (f *fake) set(v float64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.value, f.err = v, err
}

// blockNext makes evaluations wait for the returned channel to be closed.
func (f *fake) blockNext() chan struct{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.block, f.started = make(chan struct{}), make(chan struct{}, 10)
	return f.block
}

func (f *fake) unblock() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.block = nil
}

var (
	ff  = &fake{}
	key = bytes.Repeat([]byte{1}, 32)
)

func init() {
	datasource.Register("fake", ff)
}

type fixture struct {
	db    *store.DB
	p     *Poller
	site  *model.Site
	ds    *model.DataSource
	panel *model.Panel
}

func newFixture(t *testing.T, refresh time.Duration) *fixture {
	t.Helper()
	require := require.New(t)

	ff.set(1, nil)
	ff.unblock()

	db, err := store.Open(filepath.Join(t.TempDir(), "sitrep.db"))
	require.NoError(err)
	t.Cleanup(func() { _ = db.Close() })

	f := &fixture{
		db: db,
		p:  New(xlog.NewDiscard(), db, key, refresh),
	}
	f.site = &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	require.NoError(db.CreateSite(f.site))
	f.ds = &model.DataSource{
		ID:   "ds",
		Type: "fake",
	}
	require.NoError(db.CreateDataSource(f.ds))
	f.panel = &model.Panel{
		Site:       f.site.ID,
		Type:       model.PanelStat,
		DataSource: "ds",
	}
	require.NoError(db.CreatePanel(f.panel))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		f.p.Run(ctx)
		close(done)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})
	return f
}

// await waits for the panel's entry to satisfy cond.
func (f *fixture) await(t *testing.T, cond func(Entry) bool) Entry {
	t.Helper()
	var e Entry
	ready := func() bool {
		var ok bool
		e, ok = f.p.Entry(f.panel.ID)
		return ok && cond(e)
	}

	require.Eventually(t, ready, 5*time.Second, time.Millisecond)
	return e
}

func fresh(e Entry) bool { return !e.Pending() && !e.Stale }

func TestPollAndFailure(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, time.Hour)
	e := f.await(t, fresh)
	assert.Equal(1.0, *e.Data.Value)

	ff.set(2, errors.New("backend down"))
	f.p.SiteChanged(f.site.ID) // a site change alone does not restart polls
	got, _ := f.p.Entry(f.panel.ID)
	assert.Equal(e, got)

	// restarts the poll with the failure
	require.NoError(f.db.UpdateDataSource(f.ds))
	f.p.Reconcile()
	stale := f.await(t, func(e Entry) bool { return e.Err != "" })
	assert.True(
		stale.Pending(),
		"a definition change discards the data, "+
			"and a failure without data stays pending",
	)
	assert.False(stale.Stale)
	assert.Equal("backend down", stale.Err)
}

func TestStaleKeepsData(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t, 10*time.Millisecond)
	ok := f.await(t, fresh)

	ff.set(2, errors.New("backend down"))
	stale := f.await(t, func(e Entry) bool { return e.Stale })
	assert.Equal(ok.Data, stale.Data, "a failure keeps the previous data")
	assert.Equal(ok.FetchedAt, stale.FetchedAt)
	assert.Equal("backend down", stale.Err)
	assert.Greater(stale.Seq, ok.Seq, "becoming stale is a visible change")

	time.Sleep(50 * time.Millisecond)
	again, _ := f.p.Entry(f.panel.ID)
	assert.Equal(stale.Seq, again.Seq, "further failures change nothing visible")

	ff.set(3, nil)
	e := f.await(t, fresh)
	assert.Equal(3.0, *e.Data.Value)
	assert.Empty(e.Err, "success clears the error")
}

func TestDefinitionChangeDiscards(t *testing.T) {
	f := newFixture(t, time.Hour)
	prev := f.await(t, fresh)

	block := ff.blockNext()
	defer close(block)

	for name, change := range map[string]func() error{
		"panel":       func() error { return f.db.UpdatePanel(f.panel) },
		"data source": func() error { return f.db.UpdateDataSource(f.ds) },
	} {
		require.NoError(t, change())
		f.p.Reconcile()
		e, _ := f.p.Entry(f.panel.ID)
		assert.True(t, e.Pending(), name)
		assert.Greater(t, e.Seq, prev.Seq, name)
		<-ff.started
		prev = e
	}
}

func TestSupersededPollIsDiscarded(t *testing.T) {
	f := newFixture(t, time.Hour)
	f.await(t, fresh)

	block := ff.blockNext()
	defer close(block)

	f.p.mu.Lock()
	old := f.p.jobs[f.panel.ID]
	f.p.mu.Unlock()

	require.NoError(t, f.db.UpdatePanel(f.panel))
	f.p.Reconcile()
	v := 9.0
	f.p.store(old, *f.panel, process.Data{Value: &v}, nil)
	e, _ := f.p.Entry(f.panel.ID)
	assert.True(
		t,
		e.Pending(),
		"the result of a poll for the old definition is discarded",
	)
}

func TestDeletedPanelStops(t *testing.T) {
	f := newFixture(t, time.Hour)
	f.await(t, fresh)

	f.p.mu.Lock()
	old := f.p.jobs[f.panel.ID]
	f.p.mu.Unlock()

	require.NoError(t, f.db.DeletePanel(f.site.ID, f.panel.ID))
	f.p.SiteChanged(f.site.ID)
	_, ok := f.p.Entry(f.panel.ID)
	assert.False(t, ok)

	f.p.store(old, *f.panel, process.Data{}, nil)
	_, ok = f.p.Entry(f.panel.ID)
	assert.False(t, ok, "results of stopped polls are discarded")
}

func TestPausedSiteStops(t *testing.T) {
	f := newFixture(t, time.Hour)
	f.await(t, fresh)

	f.site.Availability = model.AvailabilityPaused
	require.NoError(t, f.db.UpdateSite(f.site))
	f.p.SiteChanged(f.site.ID)
	_, ok := f.p.Entry(f.panel.ID)
	assert.False(t, ok, "polling stops and the entry is discarded")

	f.site.Availability = model.AvailabilityOffline
	require.NoError(t, f.db.UpdateSite(f.site))
	f.p.SiteChanged(f.site.ID)
	f.await(t, fresh)
}

func TestShutdownWaitsForPolls(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	db, err := store.Open(filepath.Join(t.TempDir(), "sitrep.db"))
	require.NoError(err)
	t.Cleanup(func() { _ = db.Close() })

	site := &model.Site{Route: model.Route{
		Mode: model.RoutePath,
		Slug: "a",
	}}
	require.NoError(db.CreateSite(site))
	require.NoError(db.CreateDataSource(&model.DataSource{
		ID:   "ds",
		Type: "fake",
	}))
	require.NoError(db.CreatePanel(&model.Panel{
		Site:       site.ID,
		Type:       model.PanelStat,
		DataSource: "ds",
	}))

	ff.blockNext()
	t.Cleanup(ff.unblock)
	p := New(xlog.NewDiscard(), db, key, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		p.Run(ctx)
		close(done)
	}()

	<-ff.started
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}

	assert.Empty(p.jobs)
	p.Reconcile()
	assert.Empty(p.jobs, "no polls start after shutdown")
}

func TestUnusableDataSource(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	f := newFixture(t, time.Hour)
	var errs apierr.Fields
	config := map[string]string{"token": "t0ken"}
	otherKey := bytes.Repeat([]byte{2}, 32)
	require.NoError(datasource.Apply(&errs, f.ds, config, otherKey))
	require.Empty(errs)
	require.NoError(f.db.UpdateDataSource(f.ds))
	f.p.Reconcile()
	e := f.await(t, func(e Entry) bool { return e.Err != "" })
	assert.Equal(datasource.ErrUnusable.Error(), e.Err)
}

func TestCursor(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t, time.Hour)
	f.await(t, fresh)

	seq := f.p.Seq()
	cursor := f.p.Cursor(seq)
	got, ok := f.p.Since(cursor)
	assert.True(ok)
	assert.Equal(seq, got)
	for _, c := range []string{
		"",
		"x",
		f.p.boot,
		f.p.boot + ".",
		f.p.boot + ".x",
		f.p.boot + ".-1",
		"other." + "1",
		f.p.Cursor(seq + 1),
	} {
		_, ok := f.p.Since(c)
		assert.False(ok, c)
	}

	f.p.SiteChanged(f.site.ID)
	assert.Equal(seq+1, f.p.SiteSeq(f.site.ID))
	assert.Zero(f.p.SiteSeq("other"))
	f.p.SettingsChanged()
	assert.Equal(
		seq+2,
		f.p.SiteSeq("other"),
		"instance settings changes affect every site",
	)
}

func TestIncidentsSeq(t *testing.T) {
	assert := assert.New(t)

	f := newFixture(t, time.Hour)
	f.await(t, fresh)

	p := f.p
	site := f.site.ID
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	assert.Zero(p.IncidentsSeq("other", now))

	p.IncidentsChanged(site)
	seq := p.Seq()
	assert.Equal(seq, p.IncidentsSeq(site, now))
	p.SettingsChanged()
	assert.Equal(
		seq,
		p.IncidentsSeq(site, now),
		"instance settings do not affect the incidents' view",
	)
	p.SiteChanged(site)
	assert.Equal(p.SiteSeq(site), p.IncidentsSeq(site, now), "site changes do")
	seq = p.Seq()

	p.ExpireIncidents(site, now.Add(time.Hour))
	p.ExpireIncidents(site, now.Add(2*time.Hour))
	assert.Equal(seq, p.IncidentsSeq(site, now.Add(time.Hour-time.Second)))
	assert.Equal(
		seq+1,
		p.IncidentsSeq(site, now.Add(time.Hour)),
		"the earliest expiry counts as a change",
	)
	assert.Equal(
		seq+1,
		p.IncidentsSeq(site, now.Add(3*time.Hour)),
		"an expiry counts once",
	)
}
