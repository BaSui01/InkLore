package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"InkLore/internal/platform/database"
	r1schema "InkLore/migrations/r1"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// r1InsertPlan uses explicit target columns rather than GORM's inferred legacy struct layout.
// Unsupported source shapes must fail BEFORE opening the write transaction.
type r1InsertPlan struct {
	table string
	rows  []map[string]interface{}
}

type r1Row = map[string]interface{}

// pgArrayValue is a bound PostgreSQL text[] literal, never interpolated into SQL.
func pgArrayValue(values []string) interface{} {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
	}
	return gorm.Expr("?::text[]", "{"+strings.Join(parts, ",")+"}")
}

func r1MissingData(r1 *R1Dataset) []string {
	var gaps []string
	if r1 == nil {
		return []string{"R1Dataset 为空"}
	}
	if len(r1.Tenants) == 0 || len(r1.Users) == 0 {
		gaps = append(gaps, "platform_tenants/platform_users: 缺可验证的原始身份，禁止空集合标记迁移完成")
	}
	// No invented foreign keys, pricing, policies or job/definition versions. Never drop
	// these slices simply because the physical table can otherwise be INSERTed.
	unsupported := []struct {
		table, reason string
		count         int
	}{
		{"llm_models", "缺 tenant_id、capabilities、context_window、max_output_tokens", len(r1.LLMModels)},
		{"llm_channels", "缺 tenant_id、model_id、upstream_model、rpm、concurrency", len(r1.LLMChannels)},
		{"llm_keys", "缺 tenant_id、secret_kind/locator/version；原 SecretRef 未经可信校验", len(r1.LLMKeys)},
		{"llm_usage_records", "缺 reservation_id、policy_version_id、pricing_revision_id、business_date、price_snapshot 等不可伪造计量字段", len(r1.LLMUsageRecords)},
		{"agent_definitions", "缺任务类型 task_kinds 和可信工具版本绑定，不能生成可用定义版本", len(r1.AgentDefinitions)},
		{"agent_runs", "缺 owner_user_id、platform_job_id、policy_version_id、deadline_at、instruction 与运行快照", len(r1.AgentRuns)},
	}
	for _, item := range unsupported {
		if item.count > 0 {
			gaps = append(gaps, fmt.Sprintf("%s: %s (%d 行)", item.table, item.reason, item.count))
		}
	}
	for _, user := range r1.Users {
		// Transformer currently emits this public, nonfunctional placeholder for every
		// user. Importing it would create an unsafe account with no real credential.
		if user.Email == "system-creator@inklore.local" {
			gaps = append(gaps, "platform_users: 缺旧库真实用户，转换器生成的默认作者不可作为迁移身份")
			break
		}
		if user.PasswordHash == "$2a$12$e8wWwF3qF9l9p4Z5Z6h7iO9j0k1l2m3n4o5p6q7r8s9t0u1v2w3x" {
			gaps = append(gaps, "platform_users: transformer 使用固定伪造密码哈希，须提供真实可信的凭据迁移/重置流程")
			break
		}
		cost, err := bcrypt.Cost([]byte(user.PasswordHash))
		if err != nil || cost < 12 {
			gaps = append(gaps, "platform_users: 密码哈希不是可验证的 bcrypt cost>=12")
			break
		}
	}
	if len(r1.Novels) > 0 && (len(r1.Users) == 0 || len(r1.Tenants) == 0) {
		gaps = append(gaps, "content_novels: 缺可验证的租户或用户身份映射")
	}
	if len(r1.LedgerEvents) > 0 && len(r1.FactPayloads) != len(r1.LedgerEvents) {
		gaps = append(gaps, "worldview_fact_payloads: 事件与载荷行数不一致")
	}
	memberships := make(map[string]bool, len(r1.Memberships))
	for _, m := range r1.Memberships {
		memberships[m.TenantID.String()+"/"+m.UserID.String()] = true
	}
	for _, n := range r1.Novels {
		if !memberships[n.TenantID.String()+"/"+n.OwnerID.String()] {
			gaps = append(gaps, "content_novels: 作品 owner 缺匹配租户的成员关系")
			break
		}
	}
	for _, c := range r1.Chapters {
		if c.ContentHash != ComputeContentHash(c.Body) || r1NonspaceCount(c.Body) != c.WordCount {
			gaps = append(gaps, "content_chapters: 正文 SHA256 或非空白码点字数与冻结数据不一致")
			break
		}
	}
	for _, v := range r1.ChapterVersions {
		if v.ContentHash != ComputeContentHash(v.Body) || r1NonspaceCount(v.Body) != v.WordCount {
			gaps = append(gaps, "content_chapter_versions: 快照 SHA256 或非空白码点字数与冻结数据不一致")
			break
		}
	}
	return gaps
}

func r1NonspaceCount(body string) int {
	count := 0
	for _, r := range body {
		if !unicode.IsSpace(r) {
			count++
		}
	}
	return count
}

// buildR1InsertPlans is deliberately ordered by FK dependency; chapter/current
// version circular references are deferred by the frozen R1 migration.
func buildR1InsertPlans(r1 *R1Dataset) []r1InsertPlan {
	plans := []r1InsertPlan{
		{table: "platform_tenants"}, {table: "platform_users"}, {table: "platform_memberships"},
		{table: "content_novels"}, {table: "platform_resource_scopes"}, {table: "platform_resource_revisions"},
		{table: "content_volumes"}, {table: "content_chapters"}, {table: "content_chapter_versions"},
		{table: "outline_nodes"}, {table: "outline_scenes"},
		{table: "worldview_revisions"}, {table: "worldview_entities"}, {table: "worldview_relations"},
		{table: "worldview_fact_ledger_events"}, {table: "worldview_fact_payloads"}, {table: "worldview_fact_heads"},
		{table: "llm_providers"},
	}
	appendRow := func(index int, row r1Row) { plans[index].rows = append(plans[index].rows, row) }
	for _, t := range r1.Tenants {
		appendRow(0, r1Row{"id": t.ID, "name": t.Name, "status": t.Status, "revision": t.Revision, "created_at": t.CreatedAt, "updated_at": t.UpdatedAt})
	}
	for _, u := range r1.Users {
		appendRow(1, r1Row{"id": u.ID, "email": u.Email, "display_name": u.DisplayName, "password_hash": u.PasswordHash, "status": u.Status, "revision": u.Revision, "created_at": u.CreatedAt, "updated_at": u.UpdatedAt})
	}
	for _, m := range r1.Memberships {
		appendRow(2, r1Row{"id": m.ID, "tenant_id": m.TenantID, "user_id": m.UserID, "roles": pgArrayValue(m.Roles), "status": m.Status, "revision": m.Revision, "created_at": m.CreatedAt, "updated_at": m.UpdatedAt})
	}
	for _, n := range r1.Novels {
		appendRow(3, r1Row{"id": n.ID, "tenant_id": n.TenantID, "owner_id": n.OwnerID, "title": n.Title, "summary": n.Summary, "category": n.Category, "status": n.Status, "target_words": n.TargetWords, "word_count": n.WordCount, "revision": n.Revision, "created_at": n.CreatedAt, "updated_at": n.UpdatedAt})
	}
	for _, s := range r1.ResourceScopes {
		appendRow(4, r1Row{"tenant_id": s.TenantID, "resource_id": s.ResourceID, "resource_kind": s.ResourceKind, "owner_id": s.OwnerID, "novel_id": s.NovelID, "current_revision": s.CurrentRevision, "content_hash": s.ContentHash, "revision": s.Revision, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt})
	}
	for _, v := range r1.ResourceRevisions {
		length := v.ContentLength
		for _, n := range r1.Novels {
			if n.ID == v.ResourceID && n.TenantID == v.TenantID && v.ContentHash == ComputeContentHash(n.Title+"\n"+n.Summary) {
				// The transformer hashes title + LF + summary, but omits LF from length.
				length = int64(len(n.Title + "\n" + n.Summary))
				break
			}
		}
		appendRow(5, r1Row{"tenant_id": v.TenantID, "resource_id": v.ResourceID, "revision": v.Revision, "content_hash": v.ContentHash, "content_length": length, "created_at": v.CreatedAt})
	}
	for _, v := range r1.Volumes {
		appendRow(6, r1Row{"id": v.ID, "tenant_id": v.TenantID, "novel_id": v.NovelID, "title": v.Title, "summary": v.Summary, "position": v.Position, "revision": v.Revision, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt})
	}
	for _, c := range r1.Chapters {
		appendRow(7, r1Row{"id": c.ID, "tenant_id": c.TenantID, "novel_id": c.NovelID, "volume_id": c.VolumeID, "title": c.Title, "position": c.Position, "status": c.Status, "body": c.Body, "current_version_id": c.CurrentVersionID, "confirmed_version_id": c.ConfirmedVersionID, "content_hash": c.ContentHash, "word_count": c.WordCount, "revision": c.Revision, "created_at": c.CreatedAt, "updated_at": c.UpdatedAt})
	}
	for _, v := range r1.ChapterVersions {
		appendRow(8, r1Row{"id": v.ID, "tenant_id": v.TenantID, "novel_id": v.NovelID, "chapter_id": v.ChapterID, "chapter_revision": v.ChapterRevision, "title": v.Title, "body": v.Body, "content_hash": v.ContentHash, "word_count": v.WordCount, "source": v.Source, "actor_id": v.ActorID, "created_at": v.CreatedAt, "updated_at": v.UpdatedAt})
	}
	for _, n := range r1.OutlineNodes {
		appendRow(9, r1Row{"id": n.ID, "tenant_id": n.TenantID, "novel_id": n.NovelID, "parent_id": n.ParentID, "level": n.Level, "target_id": n.TargetID, "title": n.Title, "summary": n.Summary, "content_hash": n.ContentHash, "position": n.Position, "status": n.Status, "revision": n.Revision, "created_at": n.CreatedAt, "updated_at": n.UpdatedAt})
	}
	for _, s := range r1.OutlineScenes {
		appendRow(10, r1Row{"id": s.ID, "tenant_id": s.TenantID, "novel_id": s.NovelID, "node_id": s.NodeID, "title": s.Title, "summary": s.Summary, "position": s.Position, "status": s.Status, "revision": s.Revision, "created_at": s.CreatedAt, "updated_at": s.UpdatedAt})
	}
	for _, w := range r1.WorldviewRevisions {
		appendRow(11, r1Row{"id": w.ID, "tenant_id": w.TenantID, "novel_id": w.NovelID, "revision": w.Revision, "last_seq": w.LastSeq, "created_at": w.CreatedAt, "updated_at": w.UpdatedAt})
	}
	for _, e := range r1.WorldviewEntities {
		appendRow(12, r1Row{"id": e.ID, "tenant_id": e.TenantID, "novel_id": e.NovelID, "kind": e.Kind, "name": e.Name, "description": e.Description, "aliases": pgArrayValue(e.Aliases), "role_type": e.RoleType, "life_status": e.LifeStatus, "realm_rank": e.RealmRank, "category": e.Category, "status": e.Status, "revision": e.Revision, "created_at": e.CreatedAt, "updated_at": e.UpdatedAt})
	}
	for _, r := range r1.WorldviewRelations {
		appendRow(13, r1Row{"id": r.ID, "tenant_id": r.TenantID, "novel_id": r.NovelID, "subject_id": r.SubjectID, "object_id": r.ObjectID, "predicate": r.Predicate, "description": r.Description, "weight": r.Weight, "status": r.Status, "revision": r.Revision, "created_at": r.CreatedAt, "updated_at": r.UpdatedAt})
	}
	for _, e := range r1.LedgerEvents {
		appendRow(14, r1Row{"id": e.ID, "tenant_id": e.TenantID, "novel_id": e.NovelID, "seq": e.Seq, "world_revision": e.WorldRevision, "subject_id": e.SubjectID, "predicate": e.Predicate, "fact_key": e.FactKey, "action": e.Action, "target_event_id": e.TargetEventID, "resolution": e.Resolution, "source_chapter_id": e.SourceChapterID, "source_version_id": e.SourceVersionID, "payload_digest": e.PayloadDigest, "created_at": e.CreatedAt, "updated_at": e.UpdatedAt})
	}
	for _, p := range r1.FactPayloads {
		appendRow(15, r1Row{"id": p.ID, "tenant_id": p.TenantID, "novel_id": p.NovelID, "event_id": p.EventID, "payload_salt": p.PayloadSalt, "value_kind": p.ValueKind, "value_text": p.ValueText, "value_number": p.ValueNumber, "value_boolean": p.ValueBoolean, "value_entity_id": p.ValueEntityID, "reason": p.Reason, "actor_id": p.ActorID, "source_content_hash": p.SourceContentHash, "created_at": p.CreatedAt, "updated_at": p.UpdatedAt})
	}
	for _, h := range r1.FactHeads {
		appendRow(16, r1Row{"id": h.ID, "tenant_id": h.TenantID, "novel_id": h.NovelID, "subject_id": h.SubjectID, "predicate": h.Predicate, "fact_key": h.FactKey, "event_id": h.EventID, "status": h.Status, "revision": h.Revision, "created_at": h.CreatedAt, "updated_at": h.UpdatedAt})
	}
	// Provider data has no tenant in R1Dataset; a unique target tenant is a
	// verifiable provenance only when the dataset has exactly one tenant.
	if len(r1.Tenants) == 1 {
		for _, p := range r1.LLMProviders {
			appendRow(17, r1Row{"id": p.ID, "tenant_id": r1.Tenants[0].ID, "code": p.Code, "title": p.DisplayName, "protocol": p.Protocol, "enabled": p.Enabled, "revision": int64(1), "created_at": p.CreatedAt, "updated_at": p.UpdatedAt})
		}
	}
	return plans
}

// applyR1Dataset writes only a complete, compatible R1Dataset to an empty,
// checksum-verified R1 database. No data, SQL or driver error is echoed to callers.
func applyR1Dataset(ctx context.Context, db *gorm.DB, r1 *R1Dataset) error {
	if ctx == nil || db == nil {
		return fmt.Errorf("R1 落库要求有效上下文与数据库连接")
	}
	gaps := r1MissingData(r1)
	if r1 != nil && len(r1.LLMProviders) > 0 && len(r1.Tenants) != 1 {
		gaps = append(gaps, "llm_providers: R1Dataset 未携带 tenant_id，多租户无法恢复供应商归属")
	}
	if len(gaps) > 0 {
		return fmt.Errorf("R1 数据集不可迁移（未写入数据库）: %s", strings.Join(gaps, "; "))
	}
	plans := buildR1InsertPlans(r1)
	// Query/version metadata and SQL errors can contain private data. Do not let
	// a caller-supplied logger expose INSERT bindings, even on failed writes.
	quiet := db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).WithContext(ctx)
	var safeError error
	txError := quiet.Transaction(func(tx *gorm.DB) (callbackError error) {
		defer func() { safeError = callbackError }()
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(731160)).Error; err != nil {
			return fmt.Errorf("R1 落库无法获取独占迁移锁")
		}
		if err := database.Verify(ctx, tx, r1schema.FS(), r1schema.Dir); err != nil {
			return fmt.Errorf("R1 目标迁移版本或校验和不一致，拒绝写入")
		}
		// Lock every application table before the empty check, including tables
		// added after the original 74-table catalog, preventing concurrent writes.
		var tables []string
		if err := tx.Raw("SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname=current_schema() AND tablename <> 'schema_migrations' ORDER BY tablename").Scan(&tables).Error; err != nil {
			return fmt.Errorf("R1 目标表清单无法读取")
		}
		validName := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
		for _, table := range tables {
			if !validName.MatchString(table) {
				return fmt.Errorf("R1 目标表名不符合迁移约定")
			}
			if err := tx.Exec("LOCK TABLE " + table + " IN ACCESS EXCLUSIVE MODE").Error; err != nil {
				return fmt.Errorf("R1 目标表 %s 无法独占锁定", table)
			}
			var count int64
			if err := tx.Table(table).Count(&count).Error; err != nil {
				return fmt.Errorf("R1 目标表 %s 无法验证为空", table)
			}
			if count != 0 {
				return fmt.Errorf("R1 目标表 %s 已存在数据，拒绝覆盖", table)
			}
		}
		known := make(map[string]bool, len(tables))
		for _, table := range tables {
			known[table] = true
		}
		for _, plan := range plans {
			if !known[plan.table] {
				return fmt.Errorf("R1 缺少目标表 %s", plan.table)
			}
			if len(plan.rows) == 0 {
				continue
			}
			var names []string
			if err := tx.Raw("SELECT column_name FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=?", plan.table).Scan(&names).Error; err != nil {
				return fmt.Errorf("R1 目标表 %s 字段核验失败", plan.table)
			}
			columns := make(map[string]bool, len(names))
			for _, name := range names {
				columns[name] = true
			}
			for name := range plan.rows[0] {
				if !columns[name] {
					return fmt.Errorf("R1 目标表 %s 缺字段 %s", plan.table, name)
				}
			}
		}
		for _, plan := range plans {
			if len(plan.rows) == 0 {
				continue
			}
			// PostgreSQL's 65535 bind parameter ceiling includes every column of
			// every row. Keep batches small enough for both wide and large tables.
			batch := 512
			if perRow := len(plan.rows[0]); perRow > 0 && 32000/perRow < batch {
				batch = 32000 / perRow
			}
			if batch < 1 {
				batch = 1
			}
			for start := 0; start < len(plan.rows); start += batch {
				end := start + batch
				if end > len(plan.rows) {
					end = len(plan.rows)
				}
				result := tx.Table(plan.table).CreateInBatches(plan.rows[start:end], batch)
				if result.Error != nil || result.RowsAffected != int64(end-start) {
					return fmt.Errorf("R1 表 %s 批量插入失败，已回滚", plan.table)
				}
			}
			var actual int64
			if err := tx.Table(plan.table).Count(&actual).Error; err != nil || actual != int64(len(plan.rows)) {
				return fmt.Errorf("R1 表 %s 落库行数核验失败，已回滚", plan.table)
			}
		}
		return nil
	})
	if txError != nil {
		if safeError != nil {
			return safeError
		}
		return fmt.Errorf("R1 事务启动或提交失败，数据库诊断已隐藏")
	}
	return nil
}
