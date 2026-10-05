package model

import (
	"cmp"
	"net/http"
	"slices"
	"time"
	"uuid"

	"github.com/digineo/sitrep/internal/apierr"
)

// Incident statuses.
const (
	StatusPlanned       = "planned"
	StatusActive        = "active"
	StatusInvestigating = "investigating"
	StatusMonitoring    = "monitoring"
	StatusResolved      = "resolved"
)

// Incident severities.
const (
	SeverityMinor    = "minor"
	SeverityMajor    = "major"
	SeverityCritical = "critical"
)

// Incident phases, derived from the current status.
const (
	PhaseUpcoming = "upcoming"
	PhaseOngoing  = "ongoing"
	PhaseFinished = "finished"
)

var (
	statuses = []string{
		StatusPlanned,
		StatusActive,
		StatusInvestigating,
		StatusMonitoring,
		StatusResolved,
	}
	severities = []string{SeverityMinor, SeverityMajor, SeverityCritical}
	ongoing    = []string{StatusActive, StatusInvestigating, StatusMonitoring}
)

// PublicAfterFinish is how long a finished incident stays public after its
// last activity.
const PublicAfterFinish = 7 * 24 * time.Hour

const maxMarkdown = 50000

// Person is the admin who created or edited something. People are personal
// data: they are only shown in the admin console.
type Person struct {
	Subject     string `json:"subject"`
	DisplayName string `json:"displayName"`
}

// System stands in for the people of deleted accounts.
var System = Person{
	Subject:     uuid.Nil().String(),
	DisplayName: "System",
}

// Incident is a manually managed event with a timeline of updates, kept
// ordered by time.
type Incident struct {
	ID        string    `json:"id"`
	Site      string    `json:"site"`
	Title     Text      `json:"title"`
	Updates   []Update  `json:"updates"`
	Author    Person    `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Update is one entry of an incident's timeline. It sets a status, a
// severity or both.
type Update struct {
	ID          string     `json:"id"`
	At          time.Time  `json:"at"`
	Status      string     `json:"status,omitempty"`
	Severity    string     `json:"severity,omitempty"`
	Description Text       `json:"description,omitempty"` // Markdown
	Author      Person     `json:"author"`
	EditedBy    *Person    `json:"editedBy,omitempty"`
	EditedAt    *time.Time `json:"editedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Validate checks a normalized update and adds errors at prefix+field.
func (u *Update) Validate(f *apierr.Fields, prefix string, l Languages) {
	if u.Status != "" && !slices.Contains(statuses, u.Status) {
		f.Add(prefix+"status", apierr.InvalidValue)
	}

	if u.Severity != "" && !slices.Contains(severities, u.Severity) {
		f.Add(prefix+"severity", apierr.InvalidValue)
	}

	if u.Status == "" && u.Severity == "" {
		f.Add(prefix+"status", apierr.StatusOrSeverity)
	}

	required := u.Status != ""
	validateText(f, prefix+"description", u.Description, l, required, maxMarkdown)
}

// ValidateTitle checks a normalized incident title.
func ValidateTitle(f *apierr.Fields, title Text, l Languages) {
	validateText(f, "title", title, l, true, maxName)
}

// Sort orders the updates by time. Updates at the same time keep the order
// in which they were created.
func (inc *Incident) Sort() {
	slices.SortStableFunc(inc.Updates, func(a, b Update) int {
		return cmp.Or(a.At.Compare(b.At), a.CreatedAt.Compare(b.CreatedAt))
	})
}

// Check checks the rules of a sorted timeline: the first update opens the
// incident as planned or active.
func (inc *Incident) Check() error {
	if len(inc.Updates) == 0 ||
		inc.Updates[0].Status != StatusPlanned &&
			inc.Updates[0].Status != StatusActive {
		return apierr.New(http.StatusBadRequest, apierr.FirstUpdateMustOpen)
	}
	return nil
}

// latest returns the latest value that the updates set with field, or "".
func (inc *Incident) latest(field func(Update) string) string {
	for _, u := range slices.Backward(inc.Updates) {
		if v := field(u); v != "" {
			return v
		}
	}
	return ""
}

// Status returns the current status: the status of the latest update that
// sets one.
func (inc *Incident) Status() string {
	return inc.latest(func(u Update) string { return u.Status })
}

// Severity returns the current severity: the severity of the latest update
// that sets one, or "".
func (inc *Incident) Severity() string {
	return inc.latest(func(u Update) string { return u.Severity })
}

// Phase returns upcoming while the incident is planned, ongoing while it
// is active, investigated or monitored, and finished once it is resolved.
func (inc *Incident) Phase() string {
	switch s := inc.Status(); {
	case s == StatusPlanned:
		return PhaseUpcoming
	case slices.Contains(ongoing, s):
		return PhaseOngoing
	}
	return PhaseFinished
}

// LastActivity returns the time of the latest update.
func (inc *Incident) LastActivity() time.Time {
	return inc.Updates[len(inc.Updates)-1].At
}

// Span returns the time range to shade in charts: from the first update
// with an ongoing status to the latest update that sets a status. While
// the incident is ongoing, until is zero: the span is open. ok is false if
// the incident never became ongoing.
func (inc *Incident) Span() (from, until time.Time, ok bool) {
	i := slices.IndexFunc(inc.Updates, func(u Update) bool {
		return slices.Contains(ongoing, u.Status)
	})
	if i < 0 {
		return time.Time{}, time.Time{}, false
	}

	if inc.Phase() != PhaseOngoing {
		for _, u := range slices.Backward(inc.Updates) {
			if u.Status != "" {
				until = u.At
				break
			}
		}
	}
	return inc.Updates[i].At, until, true
}

// Public reports whether visitors see the incident at now: while it is
// upcoming or ongoing, and for PublicAfterFinish after its last activity.
func (inc *Incident) Public(now time.Time) bool {
	cutoff := now.Add(-PublicAfterFinish)
	return inc.Phase() != PhaseFinished || inc.LastActivity().After(cutoff)
}

// Expired reports whether retention deletes the incident at now: it is
// finished, and its last activity is older than days. Zero days keep it
// forever.
func (inc *Incident) Expired(now time.Time, days int) bool {
	return days > 0 && inc.Phase() == PhaseFinished &&
		inc.LastActivity().Before(now.Add(-time.Duration(days)*24*time.Hour))
}
