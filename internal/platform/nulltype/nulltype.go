package nulltype

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type String struct {
	String string
	Valid  bool
}

func (n *String) Scan(value any) error {
	if value == nil {
		*n = String{}
		return nil
	}
	switch v := value.(type) {
	case string:
		n.String = v
	case []byte:
		n.String = string(v)
	default:
		n.String = fmt.Sprint(v)
	}
	n.Valid = true
	return nil
}

func (n String) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.String, nil
}

type Int64 struct {
	Int64 int64
	Valid bool
}

func (n *Int64) Scan(value any) error {
	if value == nil {
		*n = Int64{}
		return nil
	}
	i, err := valueToInt64(value)
	if err != nil {
		return err
	}
	n.Int64 = i
	n.Valid = true
	return nil
}

func (n Int64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Int64, nil
}

// NullableIntArg returns a SQL parameter for optional positive IDs.
// Invalid or non-positive values become NULL (nil).
func NullableIntArg(v Int64) any {
	if !v.Valid || v.Int64 <= 0 {
		return nil
	}
	return v.Int64
}

type Float64 struct {
	Float64 float64
	Valid   bool
}

func (n *Float64) Scan(value any) error {
	if value == nil {
		*n = Float64{}
		return nil
	}
	f, err := valueToFloat64(value)
	if err != nil {
		return err
	}
	n.Float64 = f
	n.Valid = true
	return nil
}

func (n Float64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Float64, nil
}

type Bool struct {
	Bool  bool
	Valid bool
}

func (n *Bool) Scan(value any) error {
	if value == nil {
		*n = Bool{}
		return nil
	}
	b, err := valueToBool(value)
	if err != nil {
		return err
	}
	n.Bool = b
	n.Valid = true
	return nil
}

func (n Bool) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Bool, nil
}

type Time struct {
	Time  time.Time
	Valid bool
}

func (n *Time) Scan(value any) error {
	if value == nil {
		*n = Time{}
		return nil
	}
	t, err := valueToTime(value)
	if err != nil {
		return err
	}
	n.Time = t
	n.Valid = true
	return nil
}

func (n Time) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Time, nil
}

func valueToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case uint:
		return uintToInt64(uint64(v))
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return uintToInt64(v)
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case []byte:
		return parseInt64String(string(v))
	case string:
		return parseInt64String(v)
	default:
		return 0, fmt.Errorf("cannot scan %T into nulltype.Int64", value)
	}
}

func parseInt64String(value string) (int64, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, nil
	}
	i, err := strconv.ParseInt(raw, 10, 64)
	if err == nil {
		return i, nil
	}
	f, ferr := strconv.ParseFloat(raw, 64)
	if ferr != nil {
		return 0, err
	}
	return int64(f), nil
}

func uintToInt64(value uint64) (int64, error) {
	if value > uint64(^uint64(0)>>1) {
		return 0, fmt.Errorf("cannot scan uint64 %d into nulltype.Int64", value)
	}
	return int64(value), nil
}

func valueToFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case []byte:
		return parseFloat64String(string(v))
	case string:
		return parseFloat64String(v)
	default:
		return 0, fmt.Errorf("cannot scan %T into nulltype.Float64", value)
	}
}

func parseFloat64String(value string) (float64, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func valueToBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case []byte:
		return parseBoolString(string(v))
	case string:
		return parseBoolString(v)
	default:
		return false, fmt.Errorf("cannot scan %T into nulltype.Bool", value)
	}
}

func parseBoolString(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "f", "false", "n", "no", "off":
		return false, nil
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	default:
		return strconv.ParseBool(value)
	}
}

func valueToTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case []byte:
		return parseTimeString(string(v))
	case string:
		return parseTimeString(v)
	default:
		return time.Time{}, fmt.Errorf("cannot scan %T into nulltype.Time", value)
	}
}

func parseTimeString(value string) (time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return time.Time{}, nil
	}
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999-07",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot scan %q into nulltype.Time", value)
}
