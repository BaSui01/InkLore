package timeutil

import "time"

// SQLiteTimeFormat is the canonical SQLite/DATETIME text layout used by persistence layers.
const SQLiteTimeFormat = "2006-01-02 15:04:05"

var beijingLocation = mustLoadLocation("Asia/Shanghai")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CST", 8*60*60)
	}
	return loc
}

// BeijingLocation returns the canonical Asia/Shanghai location used by the app.
func BeijingLocation() *time.Location {
	return beijingLocation
}

// Now returns the current absolute instant in UTC. Storage, API responses and
// WebSocket timestamps must not inherit the host machine timezone.
func Now() time.Time {
	return time.Now().UTC()
}

// BeijingNow returns the current time in the business timezone. Use it only
// when deriving a DATE period or a local reporting boundary.
func BeijingNow() time.Time {
	return Now().In(beijingLocation)
}

// InBeijing converts a time to Beijing time while preserving the instant.
func InBeijing(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(beijingLocation)
}

// FormatSQLite is retained for SQL paths shared with legacy SQLite fixtures.
// PostgreSQL sessions are forced to UTC, so this layout is always a UTC
// wall-clock representation and cannot depend on the host timezone.
func FormatSQLite(t time.Time) string {
	return t.UTC().Format(SQLiteTimeFormat)
}

// FormatSQLiteNow returns the current UTC time formatted for legacy SQL paths.
func FormatSQLiteNow() string {
	return FormatSQLite(Now())
}

// FromDB remains as a compatibility entry point for transport mappers. All
// persisted instants are now TIMESTAMPTZ, so a scan already carries its true
// instant and must never be relabeled to another timezone.
func FromDB(t time.Time) time.Time {
	return t
}

// FromDBPtr is FromDB for optional columns; nil stays nil.
func FromDBPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := FromDB(*t)
	return &v
}

// FormatAPI renders a time for API responses as UTC RFC3339. Zero times render
// as "". API timestamps remain absolute instants; Beijing conversion belongs
// exclusively to the presentation layer.
func FormatAPI(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// FormatDB serializes a database instant for HTTP transport. It is kept as the
// mapper-facing name while sharing the same UTC RFC3339 contract as FormatAPI.
func FormatDB(t time.Time) string {
	return FormatAPI(t)
}

// apiTextLayouts are wall-clock layouts persistence may hand back as text.
var apiTextLayouts = []string{SQLiteTimeFormat, "2006-01-02 15:04", "2006-01-02"}

// ParseDBText parses persistence time text. Offset-carrying strings (RFC3339)
// keep their instant; bare wall-clock text is interpreted as Beijing time.
func ParseDBText(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	var lastErr error
	for _, layout := range apiTextLayouts {
		parsed, err := time.ParseInLocation(layout, value, beijingLocation)
		if err == nil {
			return parsed, nil
		}
		lastErr = err
	}
	return time.Time{}, lastErr
}
