package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// legacySource describes only physical tables and columns established by
// migrations/000001..000018. Tenants and users are deliberately absent: that
// migration stream has no tenant/user table from which identities can be read.
// Never invent identity rows or silently treat a missing physical table as empty.
type legacySource struct {
	table   string
	columns string
	// PostgreSQL UUID and JSONB columns must arrive as text in the legacy DTOs.
	textColumns string
	// Source column names that differ from the destination DTO field names.
	aliases map[string]string
}

var legacySources = []legacySource{
	{"novels", "id title synopsis world_bible genre tenant_id created_at updated_at", "id", nil},
	{"novel_volumes", "id novel_id tenant_id title summary ordinal created_at updated_at", "id novel_id", nil},
	{"novel_chapters", "id novel_id volume_id tenant_id ordinal title outline content status created_at updated_at", "id novel_id volume_id", nil},
	{"novel_chapter_versions", "id novel_id chapter_id tenant_id revision title outline content word_count source model note created_at", "id novel_id chapter_id", nil},
	{"novel_characters", "id novel_id tenant_id name role profile created_at", "id novel_id profile", nil},
	{"fact_ledger", "id novel_id tenant_id category fact_key fact_value source_chapter_id is_active created_at", "id novel_id fact_value source_chapter_id", nil},
	{"novel_outline_nodes", "id novel_id tenant_id parent_id node_type title content ordinal created_at", "id novel_id parent_id", map[string]string{"node_type": "level"}},
	{"novel_scenes", "id novel_id chapter_id tenant_id title brief status word_count ordinal created_at updated_at", "id novel_id chapter_id", nil},
	{"llm_providers", "id code display_name protocol enabled created_at", "", nil},
	{"llm_models", "id provider_id model_key capabilities enabled", "capabilities", nil},
	{"llm_channels", "id provider_id name base_url routing_weight enabled", "", nil},
	{"llm_channel_keys", "id channel_id key_name secret_ref weight daily_limit rate_limit concurrency_limit status created_at updated_at", "", nil},
	{"llm_usage_records", "id request_id model_key channel_id input_tokens output_tokens latency_ms status created_at", "", nil},
	{"agent_definitions", "id name system_prompt default_model tools created_at", "tools", nil},
	{"agent_runs", "id agent_id tenant_id request_id status input_snapshot created_at", "input_snapshot", nil},
}

// extractLegacyDataset obtains one consistent snapshot of the actual legacy
// schema. Every physical source is required even when it has zero rows; missing
// or incompatible schema always aborts the whole extraction, never falls back
// to GenerateSampleLegacyDataset. The caller owns the database connection.
func extractLegacyDataset(ctx context.Context, db *gorm.DB) (*LegacyDataset, error) {
	if db == nil || ctx == nil {
		return nil, fmt.Errorf("旧库抽取需要有效的上下文与 GORM 连接")
	}
	result := &LegacyDataset{}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := verifyLegacySchema(tx); err != nil {
			return err
		}
		var err error
		if result.Novels, err = readLegacy[LegacyNovel](tx, legacySources[0]); err != nil {
			return err
		}
		if result.Volumes, err = readLegacy[LegacyVolume](tx, legacySources[1]); err != nil {
			return err
		}
		if result.Chapters, err = readLegacy[LegacyChapter](tx, legacySources[2]); err != nil {
			return err
		}
		if result.ChapterVersions, err = readLegacy[LegacyChapterVersion](tx, legacySources[3]); err != nil {
			return err
		}
		if result.Characters, err = readLegacy[LegacyCharacter](tx, legacySources[4]); err != nil {
			return err
		}
		if result.FactLedgers, err = readLegacy[LegacyFactLedger](tx, legacySources[5]); err != nil {
			return err
		}
		if result.OutlineNodes, err = readLegacy[LegacyOutlineNode](tx, legacySources[6]); err != nil {
			return err
		}
		if result.Scenes, err = readLegacy[LegacyScene](tx, legacySources[7]); err != nil {
			return err
		}
		if result.LLMProviders, err = readLegacy[LegacyLLMProvider](tx, legacySources[8]); err != nil {
			return err
		}
		if result.LLMModels, err = readLegacy[LegacyLLMModel](tx, legacySources[9]); err != nil {
			return err
		}
		if result.LLMChannels, err = readLegacy[LegacyLLMChannel](tx, legacySources[10]); err != nil {
			return err
		}
		// secret_ref is a reference name/path, NOT the secret itself. Select
		// explicit columns only: never read key material or use SELECT *.
		if result.LLMChannelKeys, err = readLegacy[LegacyLLMChannelKey](tx, legacySources[11]); err != nil {
			return err
		}
		if result.LLMUsageRecords, err = readLegacy[LegacyLLMUsageRecord](tx, legacySources[12]); err != nil {
			return err
		}
		if result.AgentDefinitions, err = readLegacy[LegacyAgentDefinition](tx, legacySources[13]); err != nil {
			return err
		}
		if result.AgentRuns, err = readLegacy[LegacyAgentRun](tx, legacySources[14]); err != nil {
			return err
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// verifyLegacySchema checks every required relation and projected column in
// the SAME repeatable-read transaction used for data extraction. The source
// table names are fixed constants, while filter values remain bound params.
func verifyLegacySchema(tx *gorm.DB) error {
	tables := make([]string, 0, len(legacySources))
	for _, source := range legacySources {
		tables = append(tables, source.table)
	}
	var relationRows []struct {
		TableName string `gorm:"column:table_name"`
	}
	if err := tx.Table("information_schema.tables").Select("table_name").
		Where("table_schema = current_schema() AND table_type = ? AND table_name IN ?", "BASE TABLE", tables).
		Scan(&relationRows).Error; err != nil {
		return fmt.Errorf("旧库表结构探测失败（无法读取元数据）")
	}
	var columnRows []struct {
		TableName  string `gorm:"column:table_name"`
		ColumnName string `gorm:"column:column_name"`
	}
	if err := tx.Table("information_schema.columns").Select("table_name, column_name").
		Where("table_schema = current_schema() AND table_name IN ?", tables).
		Scan(&columnRows).Error; err != nil {
		return fmt.Errorf("旧库列结构探测失败（无法读取元数据）")
	}
	foundTables := make(map[string]bool, len(relationRows))
	for _, row := range relationRows {
		foundTables[row.TableName] = true
	}
	foundColumns := make(map[string]map[string]bool)
	for _, row := range columnRows {
		if foundColumns[row.TableName] == nil {
			foundColumns[row.TableName] = make(map[string]bool)
		}
		foundColumns[row.TableName][row.ColumnName] = true
	}
	for _, source := range legacySources {
		if !foundTables[source.table] {
			return fmt.Errorf("旧库缺少必需数据表 %s（请先应用旧版迁移，不能使用样本代替）", source.table)
		}
		for _, col := range strings.Fields(source.columns) {
			if !foundColumns[source.table][col] {
				return fmt.Errorf("旧库数据表 %s 缺少必需列 %s（不能使用样本代替）", source.table, col)
			}
		}
	}
	return nil
}

func readLegacy[T any](tx *gorm.DB, source legacySource) ([]T, error) {
	cast := make(map[string]bool)
	for _, col := range strings.Fields(source.textColumns) {
		cast[col] = true
	}
	projections := make([]string, 0, len(strings.Fields(source.columns)))
	for _, col := range strings.Fields(source.columns) {
		// Identifiers originate exclusively in the fixed legacySources list.
		expr := `"` + col + `"`
		if cast[col] {
			expr += "::text"
		}
		if alias := source.aliases[col]; alias != "" {
			expr += ` AS "` + alias + `"`
		}
		projections = append(projections, expr)
	}
	var rows []T
	if err := tx.Table(source.table).Select(strings.Join(projections, ", ")).Scan(&rows).Error; err != nil {
		// Do not include a DB driver error here: row values, SQL or credentials
		// from an untrusted legacy schema must never leak into CLI reports.
		return nil, fmt.Errorf("旧库数据表 %s 读取失败（列类型或数据不兼容）", source.table)
	}
	return rows, nil
}
