package database

import (
	"fmt"
	"strings"
)

// migrationBody removes only the standalone outer transaction wrapper used by
// historical SQL files. The original bytes still determine the checksum.
// GORM owns the transaction and advisory lock; an SQL COMMIT must not release it.
func migrationBody(script string) (string, error) {
	lines := strings.Split(script, "\n")
	first, last := -1, -1
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if first < 0 {
			first = i
		}
		last = i
	}
	if first < 0 {
		return script, nil
	}
	command := func(line string) string {
		if comment := strings.Index(line, "--"); comment >= 0 {
			line = line[:comment]
		}
		return strings.ToUpper(strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(line), ";")), " "))
	}
	start, end := command(lines[first]), command(lines[last])
	wrapped := start == "BEGIN" || start == "BEGIN WORK" || start == "BEGIN TRANSACTION" || start == "START TRANSACTION"
	committed := end == "COMMIT" || end == "COMMIT WORK" || end == "COMMIT TRANSACTION" || end == "END" || end == "END WORK" || end == "END TRANSACTION"
	if !wrapped && !committed {
		if strings.HasPrefix(start, "BEGIN;") || strings.HasPrefix(start, "START TRANSACTION;") {
			return "", fmt.Errorf("migration transaction wrapper must use standalone BEGIN/COMMIT lines")
		}
		return script, nil
	}
	if !wrapped || !committed || first == last {
		return "", fmt.Errorf("migration transaction wrapper requires matching BEGIN and COMMIT")
	}
	lines[first], lines[last] = "", ""
	return strings.Join(lines, "\n"), nil
}
