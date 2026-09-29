package audit

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "InkLore/internal/platform/errors"
	"InkLore/internal/platform/timeutil"
)

const DefaultLimit = 20
const MaxLimit = 100

// ParseGETQuery never decodes or trusts Cursor. The caller must verify it before
// assigning BeforeCreatedAt/BeforeID. From and To are instants, not bare dates.
func ParseGETQuery(q GETQuery) (ListQuery, error) {
	out := ListQuery{Action: q.Action, Limit: DefaultLimit}
	for _, input := range []struct {
		text   string
		target **time.Time
	}{{q.From, &out.From}, {q.To, &out.To}} {
		if input.text == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, input.text)
		if err != nil || !strings.HasSuffix(input.text, "Z") {
			return ListQuery{}, invalid()
		}
		t = t.UTC()
		*input.target = &t
	}
	if q.Limit != "" {
		n, err := strconv.Atoi(q.Limit)
		if err != nil || n < 1 || n > MaxLimit {
			return ListQuery{}, invalid()
		}
		out.Limit = n
	}
	if err := validateQuery(out); err != nil {
		return ListQuery{}, err
	}
	return out, nil
}

func validateQuery(q ListQuery) error {
	if q.Limit < 0 || q.Limit > MaxLimit || !utf8.ValidString(q.Action) || utf8.RuneCountInString(q.Action) > 100 {
		return invalid()
	}
	if q.From != nil && q.From.IsZero() || q.To != nil && q.To.IsZero() {
		return invalid()
	}
	if q.From != nil && q.To != nil && q.From.After(*q.To) {
		return invalid()
	}
	if (q.BeforeCreatedAt == nil) != (q.BeforeID == "") || q.BeforeCreatedAt != nil && (q.BeforeCreatedAt.IsZero() || !validID(q.BeforeID)) {
		return apperrors.Contract(400, "platform.cursor_invalid", nil)
	}
	return nil
}

func authorizeReader(p VerifiedReader) error {
	if !validID(p.TenantID) || !validID(p.ActorID) {
		return apperrors.Contract(401, "auth.unauthenticated", nil)
	}
	if p.Audience == "admin" {
		for _, role := range p.Roles {
			if role == "operator" || role == "auditor" {
				return nil
			}
		}
	}
	return apperrors.Contract(403, "auth.forbidden", nil)
}

// retentionWindow is a rolling 90-calendar-day window in the business timezone.
// Its UTC bounds are also applied to Get, regardless of caller-supplied filters.
func retentionWindow(now time.Time) (time.Time, time.Time) {
	return timeutil.InBeijing(now).AddDate(0, 0, -90).UTC(), now.UTC()
}
