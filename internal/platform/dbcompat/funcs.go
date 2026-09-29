package dbcompat

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// ============================================================
// SQL 方言函数抽象层
//
// 用法：在 repository SQL 中调用这些函数代替直接写 SQLite 方言，
// 它们根据 CurrentDriver() 返回驱动感知的 SQL 片段。
// ============================================================

// TodayRangeCondition 返回可走时间列索引的"当天"范围条件。
//
//	SQLite:   column >= date('now', 'localtime') AND column < datetime(date('now', 'localtime'), '+1 day')
//	Postgres: column >= CURRENT_DATE AND column < CURRENT_DATE + INTERVAL '1 day'
func TodayRangeCondition(column string) string {
	if IsPostgres(CurrentDriver()) {
		return fmt.Sprintf("(%s >= CURRENT_DATE AND %s < CURRENT_DATE + INTERVAL '1 day')", column, column)
	}
	return fmt.Sprintf("(%s >= date('now', 'localtime') AND %s < datetime(date('now', 'localtime'), '+1 day'))", column, column)
}

// WeekdayExpr 返回星期几表达式（周一=1 … 周日=7）
//
//	SQLite:   ((CAST(strftime('%w', col) AS INTEGER) + 6) % 7) + 1
//	Postgres: ((EXTRACT(DOW FROM col)::int + 6) % 7) + 1
func WeekdayExpr(column string) string {
	if IsPostgres(CurrentDriver()) {
		return fmt.Sprintf("((EXTRACT(DOW FROM %s)::int + 6) %% 7) + 1", column)
	}
	return fmt.Sprintf("((CAST(strftime('%%w', %s) AS INTEGER) + 6) %% 7) + 1", column)
}

// HourExpr 返回小时表达式（0-23）
//
//	SQLite:   CAST(strftime('%H', col) AS INTEGER)
//	Postgres: EXTRACT(HOUR FROM col)::int
func HourExpr(column string) string {
	if IsPostgres(CurrentDriver()) {
		return fmt.Sprintf("EXTRACT(HOUR FROM %s)::int", column)
	}
	return fmt.Sprintf("CAST(strftime('%%H', %s) AS INTEGER)", column)
}

// FormatTimestampExpr 返回时间戳格式化表达式
//
//	SQLite:   strftime('%Y-%m-%d %H:%M:%S', col)
//	Postgres: TO_CHAR(col, 'YYYY-MM-DD HH24:MI:SS')
func FormatTimestampExpr(column string) string {
	if IsPostgres(CurrentDriver()) {
		return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM-DD HH24:MI:SS')", column)
	}
	return fmt.Sprintf("strftime('%%Y-%%m-%%d %%H:%%M:%%S', %s)", column)
}

// JSONTextExpr 从 JSON/JSONB 列提取文本字段。
// path 使用 JSON path，例如 "$.model" 或 "$.gateway.model"。
func JSONTextExpr(column, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "$"
	}
	if IsPostgres(CurrentDriver()) {
		keys := strings.Split(strings.TrimPrefix(path, "$."), ".")
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			parts = append(parts, strings.ReplaceAll(key, "'", "''"))
		}
		if len(parts) == 0 {
			return fmt.Sprintf("(%s)::text", column)
		}
		if len(parts) == 1 {
			return fmt.Sprintf("(%s::jsonb ->> '%s')", column, parts[0])
		}
		return fmt.Sprintf("(%s::jsonb #>> '{%s}')", column, strings.Join(parts, ","))
	}
	return fmt.Sprintf("json_extract(%s, '%s')", column, strings.ReplaceAll(path, "'", "''"))
}

// UpsertStmt 构造 upsert（INSERT OR REPLACE / ON CONFLICT）完整 SQL
//
//	SQLite:   INSERT OR REPLACE INTO table (cols) VALUES (?...)
//	Postgres: INSERT INTO table (cols) VALUES (?...) ON CONFLICT (conflictCols) DO UPDATE SET ...
//
// columns 和 conflictCols 中的列名会被 `"` 引用，避免关键字冲突。
func UpsertStmt(table string, columns, conflictCols []string) string {
	quoted := make([]string, len(columns))
	for i, c := range columns {
		quoted[i] = quoteIdent(c)
	}
	colList := join(quoted, ", ")

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	phList := join(placeholders, ", ")

	if IsPostgres(CurrentDriver()) {
		conflictQuoted := make([]string, len(conflictCols))
		for i, c := range conflictCols {
			conflictQuoted[i] = quoteIdent(c)
		}
		conflictList := join(conflictQuoted, ", ")

		updates := make([]string, len(columns))
		for i, c := range columns {
			updates[i] = fmt.Sprintf("%s = excluded.%s", quoteIdent(c), quoteIdent(c))
		}
		setList := join(updates, ", ")

		return fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s",
			quoteIdent(table), colList, phList, conflictList, setList,
		)
	}

	// SQLite
	return fmt.Sprintf(
		"INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
		quoteIdent(table), colList, phList,
	)
}

// BoolLiteral 返回布尔值的 SQL 字面量
//
//	SQLite:   0 / 1
//	Postgres: FALSE / TRUE
func BoolLiteral(v bool) string {
	if IsPostgres(CurrentDriver()) {
		if v {
			return "TRUE"
		}
		return "FALSE"
	}
	if v {
		return "1"
	}
	return "0"
}

// BoolValue returns the parameter value expected by the current SQL driver.
func BoolValue(v bool) any {
	if IsPostgres(CurrentDriver()) {
		return v
	}
	if v {
		return 1
	}
	return 0
}

// BoolCondition returns a driver-aware equality condition for boolean columns.
func BoolCondition(column string, v bool) string {
	return fmt.Sprintf("%s = %s", column, BoolLiteral(v))
}

// ScanBool returns a sql.Scanner-compatible target for driver-specific booleans.
func ScanBool(dst *bool) any {
	return BoolScanner{dst: dst}
}

// BoolScanner scans SQLite/PostgreSQL boolean values into a bool.
type BoolScanner struct {
	dst *bool
}

func (s BoolScanner) Scan(src any) error {
	if s.dst == nil {
		return fmt.Errorf("nil bool scan destination")
	}

	switch v := src.(type) {
	case nil:
		*s.dst = false
	case bool:
		*s.dst = v
	case int64:
		*s.dst = v != 0
	case int:
		*s.dst = v != 0
	case int32:
		*s.dst = v != 0
	case []byte:
		parsed, err := parseDBBool(string(v))
		if err != nil {
			return err
		}
		*s.dst = parsed
	case string:
		parsed, err := parseDBBool(v)
		if err != nil {
			return err
		}
		*s.dst = parsed
	default:
		return fmt.Errorf("cannot scan %T into bool", src)
	}
	return nil
}

// Bool is a GORM-scannable boolean wrapper for driver-specific result shapes.
type Bool bool

func (b *Bool) Scan(src any) error {
	var value bool
	scanner := ScanBool(&value).(interface{ Scan(any) error })
	if err := scanner.Scan(src); err != nil {
		return err
	}
	*b = Bool(value)
	return nil
}

func (b Bool) Bool() bool {
	return bool(b)
}

// Value implements driver.Valuer with driver-aware parameter encoding.
func (b Bool) Value() (driver.Value, error) {
	return BoolValue(bool(b)), nil
}

func parseDBBool(v string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "f", "false", "n", "no", "off":
		return false, nil
	case "1", "t", "true", "y", "yes", "on":
		return true, nil
	default:
		return strconv.ParseBool(v)
	}
}

// Greatest 返回两个表达式中的较大值
//
//	SQLite:   MAX(expr1, expr2)
//	Postgres: GREATEST(expr1, expr2)
func Greatest(expr1, expr2 string) string {
	if IsPostgres(CurrentDriver()) {
		return fmt.Sprintf("GREATEST(%s, %s)", expr1, expr2)
	}
	return fmt.Sprintf("MAX(%s, %s)", expr1, expr2)
}

// quoteIdent 用双引号引用标识符（PG 兼容；SQLite 也接受）
func quoteIdent(name string) string {
	return `"` + name + `"`
}

func join(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	result := elems[0]
	for _, e := range elems[1:] {
		result += sep + e
	}
	return result
}
