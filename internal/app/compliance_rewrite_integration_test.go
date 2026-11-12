package app_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"

	"InkLore/internal/app/adapters"
	compapp "InkLore/internal/compliance/application"
	compdomain "InkLore/internal/compliance/domain"
	compinfra "InkLore/internal/compliance/infrastructure"
	"InkLore/internal/platform/scope"
	"github.com/google/uuid"
)

// Real PostgreSQL rules/report/rewrite services and Redis-backed Gateway use a
// local HTTP protocol fixture. This does not represent live-provider acceptance.
func TestComplianceRewriteOperationPG(t *testing.T) {
	for _, status := range []int{200, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := newAgentUsageFixture(t, status, false, `{"patches":[{"start_offset":0,"end_offset":3,"old_text":"bad","new_text":"good"}]}`)
			ctx := context.Background()
			p := scope.Principal{TenantID: f.principal.TenantID, ActorID: f.principal.UserID, SessionID: f.principal.SessionID, Audience: "creator"}
			exec := func(query string, args ...any) {
				t.Helper()
				if err := f.db.Exec(query, args...).Error; err != nil {
					t.Fatal(err)
				}
			}
			exec("UPDATE llm_task_policies SET task_key='review' WHERE tenant_id=?", p.TenantID)
			service, err := compapp.NewService(compapp.Options{
				Repositories: compinfra.NewRepositories(f.manager), UoW: f.manager,
				Scope: scope.New(f.manager), SnapshotCache: compinfra.NewMemorySnapshotCache(8),
				Generation: adapters.NewComplianceGeneration(f.realGateway, f.catalog),
			})
			if err != nil {
				t.Fatal(err)
			}
			set, err := service.CreateRuleSet(ctx, p, compdomain.RuleSetCreate{Name: "Rewrite regression"})
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.CreateRule(ctx, p, set.ID, compdomain.RuleCreate{ExpectedRevision: set.Revision, Kind: "sensitive_literal", Expression: "bad", Severity: "warning", Category: "test", Advice: "rules-only suggestion", Weight: 10, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			set, err = service.GetRuleSet(ctx, p, set.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.PublishRuleSet(ctx, p, set.ID, compdomain.Expected{ExpectedRevision: set.Revision})
			if err != nil {
				t.Fatal(err)
			}
			chapter, body := uuid.NewString(), "bad text"
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
			exec(`INSERT INTO platform_resource_scopes(tenant_id,resource_id,resource_kind,owner_id,novel_id,current_revision,content_hash) VALUES(?,?,'chapter',?,?,1,?)`, p.TenantID, chapter, p.ActorID, f.resource, hash)
			exec(`INSERT INTO platform_resource_revisions(tenant_id,resource_id,revision,content_hash,content_length) VALUES(?,?,1,?,?)`, p.TenantID, chapter, hash, len(body))
			report, err := service.Scan(ctx, p, compdomain.ScanCreate{NovelID: f.resource, ChapterID: chapter, ChapterRevision: 1, ContentHash: hash, Text: body})
			if err != nil {
				t.Fatal(err)
			}
			var issueID string
			if err := f.db.Raw("SELECT id FROM compliance_issues WHERE tenant_id=? AND report_id=?", p.TenantID, report.ID).Scan(&issueID).Error; err != nil || issueID == "" {
				t.Fatalf("real scan issue missing: %v", err)
			}
			create := func() compdomain.Rewrite {
				t.Helper()
				value, err := service.CreateRewrite(ctx, p, compdomain.RewriteCreate{ReportID: report.ID, ExpectedRevision: report.Revision, SelectedIssueIDs: []string{issueID}, MaxRounds: 1, Budget: compdomain.Budget{TotalTokens: 4000, MaxOutputTokens: 512, MaxCalls: 1}})
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			first, second := create(), create()
			inputs := []compapp.ExecutionInput{
				{TenantID: p.TenantID, ResourceID: first.ID, RunID: uuid.NewString(), ExecutionKey: uuid.NewString(), Fence: 1},
				{TenantID: p.TenantID, ResourceID: second.ID, RunID: uuid.NewString(), ExecutionKey: uuid.NewString(), Fence: 1},
			}
			for _, in := range inputs {
				result, err := service.ExecuteRewrite(ctx, in)
				if status == 500 {
					if err == nil {
						t.Fatal("failed paid generation became a deterministic success")
					}
					persisted, readErr := service.GetRewrite(ctx, p, in.ResourceID)
					if readErr != nil || persisted.State != "failed" || persisted.CandidateText != nil || len(persisted.Patches) != 0 {
						t.Fatalf("generation failure was not persisted safely: %+v %v", persisted, readErr)
					}
				} else if err != nil || result.CandidateText == nil || *result.CandidateText != "good text" || result.Usage.CallsUsed != 1 {
					t.Fatalf("real model rewrite missing: %+v %v", result, err)
				}
			}
			if f.calls.Load() != 2 {
				t.Fatalf("two distinct persisted rewrites shared one dispatch: %d", f.calls.Load())
			}
			if status == 200 {
				if _, err := service.ExecuteRewrite(ctx, inputs[0]); err != nil {
					t.Fatal(err)
				}
				if f.calls.Load() != 2 {
					t.Fatal("same rewrite replay dispatched again")
				}
			}
			var count int64
			if err := f.db.Raw("SELECT count(*) FROM llm_usage_records WHERE tenant_id=?", p.TenantID).Scan(&count).Error; err != nil || count != 2 {
				t.Fatalf("rewrite receipt cardinality: %d %v", count, err)
			}
		})
	}
}
