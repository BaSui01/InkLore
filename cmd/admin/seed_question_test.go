package main

import (
	d "InkLore/internal/agent/domain"
	infra "InkLore/internal/agent/infrastructure"
	"InkLore/internal/platform/uow"
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"testing"
)

func clarificationManifest(t *testing.T) seedManifest {
	t.Helper()
	m, err := loadSeedManifest("../../configs/agent-seeds/default.json")
	if err != nil {
		t.Fatal(err)
	}
	m.Tools = []seedTool{{Key: d.ClarificationToolKey, Title: "作者澄清", InputSchema: json.RawMessage(d.ClarificationInputSchema), OutputSchema: json.RawMessage(d.ClarificationOutputSchema), SideEffect: "read_only", TimeoutSeconds: 30}}
	for i := range m.Definitions {
		for j := range m.Definitions[i].ToolKeys {
			if m.Definitions[i].ToolKeys[j] == "platform_utc_now" {
				m.Definitions[i].ToolKeys[j] = d.ClarificationToolKey
			}
		}
	}
	return m
}
func TestClarificationSeedContract(t *testing.T) {
	manifest := clarificationManifest(t)
	if err := manifest.validate(); err != nil {
		t.Fatal(err)
	}
	manifest.Tools[0].SideEffect = "external"
	if err := manifest.validate(); err == nil {
		t.Fatal("seed enabled unauthorized side effect")
	}
	manifest = clarificationManifest(t)
	manifest.Tools[0].InputSchema = json.RawMessage(`{"type":"object","properties":{"tool_scope":{"type":"string"}}}`)
	if err := manifest.validate(); err == nil {
		t.Fatal("seed replaced registered clarification schema")
	}
	manifest = clarificationManifest(t)
	manifest.Tools = append(manifest.Tools, manifest.Tools[0])
	if err := manifest.validate(); err == nil {
		t.Fatal("duplicate tool accepted")
	}
}
func TestClarificationSeedPG(t *testing.T) {
	if os.Getenv("AGENT_QUESTION_PG_TEST") != "1" {
		t.Skip("requires disposable PG runner")
	}
	dsn := os.Getenv("DATABASE_DSN")
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Path != "/inklore_ci" {
		t.Fatal("isolated inklore_ci required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sql.Close()
	tenant := uuid.NewString()
	if err = db.Exec(`INSERT INTO platform_tenants(id,name) VALUES(?,'Clarification seed test')`, tenant).Error; err != nil {
		t.Fatal(err)
	}
	manifest := clarificationManifest(t)
	first, err := seedTenantTool(context.Background(), db, tenant, manifest)
	if err != nil {
		t.Fatal(err)
	}
	second, err := seedTenantTool(context.Background(), db, tenant, manifest)
	if err != nil || first.ToolID != second.ToolID || first.ToolVersionID != second.ToolVersionID {
		t.Fatal("seed replay changed frozen tool", err)
	}
	m, err := uow.New(db)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := infra.NewRepository(m)
	if err != nil {
		t.Fatal(err)
	}
	tool, version, err := repo.GetToolVersion(context.Background(), tenant, first.ToolVersionID)
	if err != nil {
		t.Fatal(err)
	}
	executor := infra.NewToolExecutor(repo)
	view := d.ToolView{ToolKey: tool.ToolKey, Enabled: tool.Enabled, CurrentVersionID: version.ID, InputSchema: version.InputSchema, OutputSchema: version.OutputSchema, SideEffect: version.SideEffect, ApprovalRequired: version.ApprovalRequired, TimeoutSeconds: version.TimeoutSeconds}
	if executor.ToolExecutionStatus(context.Background(), tenant, view) != d.ToolExecutionAvailable {
		t.Fatal("legally seeded clarification unavailable to scheduler")
	}
	if err = db.Exec(`UPDATE agent_tools SET enabled=false WHERE tenant_id=? AND id=?`, tenant, tool.ID).Error; err != nil {
		t.Fatal(err)
	}
	again, err := seedTenantTool(context.Background(), db, tenant, manifest)
	if err != nil || len(again.SkippedKeys) != 1 {
		t.Fatal("seed revived disabled clarification", err)
	}
}
