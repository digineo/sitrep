package model

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/digineo/sitrep/internal/apierr"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// incident returns an incident with updates of the given status and
// severity, an hour apart from t0 on, created in that order.
func incident(changes ...[2]string) Incident {
	var inc Incident
	for i, c := range changes {
		at := t0.Add(time.Duration(i) * time.Hour)
		inc.Updates = append(inc.Updates, Update{
			At:        at,
			Status:    c[0],
			Severity:  c[1],
			CreatedAt: at,
		})
	}
	return inc
}

func TestUpdateValidate(t *testing.T) {
	assert := assert.New(t)

	langs := Languages{
		Enabled: []string{"en", "de"},
		Primary: "de",
	}
	validate := func(u Update) map[string]string {
		var f apierr.Fields
		u.Validate(&f, "update.", langs)
		return fieldCodes(t, f.Err())
	}

	u := Update{
		Status:      StatusActive,
		Description: Text{"de": "Ausfall"},
	}
	assert.Nil(validate(u))

	u = Update{Severity: SeverityMajor}
	assert.Nil(validate(u), "a severity-only update needs no description")

	u = Update{
		Status:      StatusActive,
		Description: Text{"en": "Outage"},
	}
	assert.Equal(
		map[string]string{"update.description.de": "required"},
		validate(u),
		"a status needs a description in the primary language",
	)

	want := map[string]string{"update.status": "status_or_severity_required"}
	assert.Equal(want, validate(Update{}))

	u = Update{
		Status:      "down",
		Severity:    "fatal",
		Description: Text{"de": "x"},
	}
	want = map[string]string{
		"update.status":   "invalid_value",
		"update.severity": "invalid_value",
	}
	assert.Equal(want, validate(u))

	u = Update{
		Severity:    SeverityMinor,
		Description: Text{"en": strings.Repeat("ä", 50001)},
	}
	want = map[string]string{"update.description.en": "too_long"}
	assert.Equal(want, validate(u))

	var f apierr.Fields
	ValidateTitle(&f, Text{"en": "Outage"}, langs)
	assert.Equal(map[string]string{"title.de": "required"}, fieldCodes(t, f.Err()))
}

func TestIncidentRules(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	inc := incident([2]string{StatusActive, ""}, [2]string{"", SeverityMajor})
	require.NoError(inc.Check())

	for _, opening := range []string{
		StatusInvestigating,
		StatusMonitoring,
		StatusResolved,
		"",
	} {
		inc := incident([2]string{opening, SeverityMinor})
		e, ok := errors.AsType[*apierr.Error](inc.Check())
		require.True(ok, opening)
		want := apierr.Error{
			Status: http.StatusBadRequest,
			Code:   "first_update_must_open",
		}
		assert.Equal(want, *e, opening)
	}

	assert.Error((&Incident{}).Check(), "an incident without updates")

	inc = incident([2]string{StatusPlanned, ""}, [2]string{StatusResolved, ""})
	require.NoError(
		inc.Check(),
		"a cancelled maintenance goes from planned straight to resolved",
	)

	inc.Updates[1].At = t0.Add(-time.Hour)
	inc.Sort()
	assert.Error(inc.Check(), "backdating before the opening update")
}

func TestIncidentSort(t *testing.T) {
	inc := incident(
		[2]string{StatusActive, ""},
		[2]string{StatusMonitoring, ""},
		[2]string{StatusResolved, ""},
	)
	inc.Updates[0], inc.Updates[2] = inc.Updates[2], inc.Updates[0]
	// resolved, created last, at the same time as monitoring
	inc.Updates[0].At = t0.Add(time.Hour)
	inc.Sort()
	want := []string{StatusActive, StatusMonitoring, StatusResolved}
	got := []string{
		inc.Updates[0].Status,
		inc.Updates[1].Status,
		inc.Updates[2].Status,
	}
	assert.Equal(t, want, got, "equal times keep the order of creation")
}

func TestIncidentDerived(t *testing.T) {
	assert := assert.New(t)

	inc := incident([2]string{StatusPlanned, ""})
	assert.Equal(PhaseUpcoming, inc.Phase())
	assert.Empty(inc.Severity())
	_, _, ok := inc.Span()
	assert.False(ok, "no span before the incident becomes ongoing")

	inc = incident(
		[2]string{StatusPlanned, ""},
		[2]string{StatusActive, SeverityMajor},
		[2]string{"", SeverityCritical},
		[2]string{StatusMonitoring, ""},
	)
	assert.Equal(StatusMonitoring, inc.Status())
	assert.Equal(
		SeverityCritical,
		inc.Severity(),
		"the latest update that sets a severity",
	)
	assert.Equal(PhaseOngoing, inc.Phase())
	assert.Equal(t0.Add(3*time.Hour), inc.LastActivity())
	from, until, ok := inc.Span()
	assert.True(ok)
	assert.Equal(t0.Add(time.Hour), from, "from the first ongoing status")
	assert.True(until.IsZero(), "open while ongoing")

	resolved := Update{
		At:     t0.Add(4 * time.Hour),
		Status: StatusResolved,
	}
	minor := Update{
		At:       t0.Add(5 * time.Hour),
		Severity: SeverityMinor,
	}
	inc.Updates = append(inc.Updates, resolved, minor)
	assert.Equal(PhaseFinished, inc.Phase())
	assert.Equal(SeverityMinor, inc.Severity())
	assert.Equal(t0.Add(5*time.Hour), inc.LastActivity())
	_, until, _ = inc.Span()
	assert.Equal(
		t0.Add(4*time.Hour),
		until,
		"until the latest update that sets a status",
	)

	for status, phase := range map[string]string{
		StatusPlanned:       PhaseUpcoming,
		StatusActive:        PhaseOngoing,
		StatusInvestigating: PhaseOngoing,
		StatusMonitoring:    PhaseOngoing,
		StatusResolved:      PhaseFinished,
	} {
		inc := incident([2]string{StatusActive, ""}, [2]string{status, ""})
		assert.Equal(phase, inc.Phase(), status)
	}
}

func TestIncidentPublic(t *testing.T) {
	assert := assert.New(t)

	finished := incident([2]string{StatusActive, ""}, [2]string{StatusResolved, ""})
	last := finished.LastActivity()
	assert.True(finished.Public(last.Add(PublicAfterFinish - time.Nanosecond)))
	assert.False(
		finished.Public(last.Add(PublicAfterFinish)),
		"strictly within the last 7 days",
	)
	assert.True(
		finished.Public(last.Add(-time.Hour)),
		"last activity in the future",
	)

	for _, inc := range []Incident{
		incident([2]string{StatusPlanned, ""}),
		incident([2]string{StatusActive, ""}),
	} {
		assert.True(
			inc.Public(t0.Add(365*24*time.Hour)),
			"upcoming and ongoing incidents are always public",
		)
	}
}

func TestIncidentExpired(t *testing.T) {
	assert := assert.New(t)

	finished := incident([2]string{StatusActive, ""}, [2]string{StatusResolved, ""})
	last := finished.LastActivity()
	assert.False(
		finished.Expired(last.Add(24*time.Hour), 1),
		"exactly the retention period",
	)
	assert.True(
		finished.Expired(last.Add(24*time.Hour+time.Nanosecond), 1),
		"older than the retention period",
	)
	assert.False(
		finished.Expired(last.Add(1000*24*time.Hour), 0),
		"zero keeps forever",
	)

	ongoing := incident([2]string{StatusActive, ""})
	assert.False(
		ongoing.Expired(t0.Add(1000*24*time.Hour), 1),
		"ongoing incidents are never deleted",
	)

	upcoming := incident([2]string{StatusPlanned, ""})
	assert.False(
		upcoming.Expired(t0.Add(1000*24*time.Hour), 1),
		"upcoming incidents are never deleted",
	)
}
