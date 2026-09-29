package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ensureSeedComplianceRules(tx *gorm.DB, tenantID, ruleSetName string, rules []seedComplianceRule, now time.Time) error {
	var existing string
	if err := tx.Raw("SELECT id FROM compliance_rule_sets WHERE tenant_id=? AND state='published' LIMIT 1", tenantID).Scan(&existing).Error; err != nil {
		return err
	}
	if existing != "" || len(rules) == 0 {
		return nil
	}
	var ownerID string
	if err := tx.Raw("SELECT id FROM platform_users WHERE tenant_id=? ORDER BY created_at,id LIMIT 1", tenantID).Scan(&ownerID).Error; err != nil {
		return err
	}
	if ownerID == "" {
		return nil
	}
	ordered := append([]seedComplianceRule(nil), rules...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Kind != ordered[j].Kind {
			return ordered[i].Kind < ordered[j].Kind
		}
		return ordered[i].Expression < ordered[j].Expression
	})
	var canonical strings.Builder
	for _, rule := range ordered {
		if rule.Kind != "ai_phrase" || rule.Expression == "" || len([]rune(rule.Expression)) > 64 || rule.Severity != "warning" || rule.Category == "" || rule.Weight < 1 || rule.Weight > 100 {
			return fmt.Errorf("invalid compliance seed rule %q", rule.Expression)
		}
		fmt.Fprintf(&canonical, "%s|%s|%s|%s|%s|%d\n", rule.Kind, rule.Expression, rule.Severity, rule.Category, rule.Advice, rule.Weight)
	}
	sum := sha256.Sum256([]byte(canonical.String()))
	checksum := hex.EncodeToString(sum[:])
	var version int64
	if err := tx.Raw("SELECT COALESCE(MAX(version_no),0)+1 FROM compliance_rule_sets WHERE tenant_id=?", tenantID).Scan(&version).Error; err != nil {
		return err
	}
	setID := uuid.NewString()
	if err := tx.Exec("INSERT INTO compliance_rule_sets(id,tenant_id,created_at,updated_at,revision,name,version_no,state,checksum,created_by,published_at) VALUES(?,?,?,?,1,?,?,'published',?,?,?)", setID, tenantID, now, now, ruleSetName, version, checksum, ownerID, now).Error; err != nil {
		return err
	}
	for _, rule := range ordered {
		if err := tx.Exec("INSERT INTO compliance_rules(id,tenant_id,created_at,updated_at,revision,rule_set_id,kind,expression,severity,category,advice,weight,enabled) VALUES(?,?,?,?,1,?,?,?,?,?,?,?,true)", uuid.NewString(), tenantID, now, now, setID, rule.Kind, rule.Expression, rule.Severity, rule.Category, rule.Advice, rule.Weight).Error; err != nil {
			return err
		}
	}
	return nil
}
