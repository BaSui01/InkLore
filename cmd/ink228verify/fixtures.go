package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fixture struct {
	tag        string
	tenant     string
	user       string
	provider   string
	model      string
	channel    string
	key        string
	policy     string
	version1   string
	version2   string
	definition string
	defVersion string
	keyEnv     string
	keyValue   string
	routeAlias string
}

func seedFixtures(ctx context.Context, db *gorm.DB) (fixture, fixture, error) {
	var a, b fixture
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		a, err = seedFixture(tx, "a")
		if err != nil {
			return err
		}
		b, err = seedFixture(tx, "b")
		return err
	})
	return a, b, err
}

func seedFixture(db *gorm.DB, tag string) (fixture, error) {
	f := fixture{
		tag: tag, tenant: uuid.NewString(), user: uuid.NewString(), provider: uuid.NewString(),
		model: uuid.NewString(), channel: uuid.NewString(), key: uuid.NewString(),
		policy: uuid.NewString(), version1: uuid.NewString(), version2: uuid.NewString(),
		definition: uuid.NewString(), defVersion: uuid.NewString(),
		keyEnv:     "CI_INK228_" + strings.ToUpper(tag) + "_KEY",
		keyValue:   "dummy-integration-only-" + tag,
		routeAlias: "ci-tenant-" + tag + "-upstream",
	}
	if err := os.Setenv(f.keyEnv, f.keyValue); err != nil {
		return f, err
	}
	execute := func(statement string, args ...any) error {
		if err := db.Exec(statement, args...).Error; err != nil {
			return fmt.Errorf("fixture insert failed (tenant %s): %w", tag, err)
		}
		return nil
	}
	if err := execute("INSERT INTO platform_tenants(id,name) VALUES (?::uuid,?)", f.tenant, "CI Tenant "+tag); err != nil {
		return f, err
	}
	if err := execute("INSERT INTO platform_users(id,email,display_name,password_hash) VALUES (?::uuid,?,?,?)",
		f.user, "ci-"+f.user+"@example.invalid", "CI User "+tag, "ci-inert-no-auth"); err != nil {
		return f, err
	}
	if err := execute("INSERT INTO platform_memberships(tenant_id,user_id,roles) VALUES (?::uuid,?::uuid,ARRAY['creator']::text[])", f.tenant, f.user); err != nil {
		return f, err
	}
	if err := execute("INSERT INTO llm_providers(id,tenant_id,code,title,protocol,enabled) VALUES (?::uuid,?::uuid,?,?,'openai',TRUE)",
		f.provider, f.tenant, "ci_provider", "CI Provider"); err != nil {
		return f, err
	}
	if err := execute(`INSERT INTO llm_models(
		id,tenant_id,provider_id,upstream_name,title,capabilities,context_window,max_output_tokens,enabled
	) VALUES (?::uuid,?::uuid,?::uuid,'same-upstream-name','CI Model',
		ARRAY['text']::text[],4096,512,TRUE)`, f.model, f.tenant, f.provider); err != nil {
		return f, err
	}
	if err := execute(`INSERT INTO llm_channels(
		id,tenant_id,provider_id,model_id,title,base_url,upstream_model,priority,weight,rpm,concurrency,enabled
	) VALUES (?::uuid,?::uuid,?::uuid,?::uuid,'CI Channel',?, ?, 1,100,100,5,TRUE)`,
		f.channel, f.tenant, f.provider, f.model, "https://ci-"+tag+".invalid/v1", f.routeAlias); err != nil {
		return f, err
	}
	if err := execute(`INSERT INTO llm_keys(
		id,tenant_id,channel_id,title,secret_kind,secret_locator,secret_version,
		fingerprint,weight,rpm,concurrency,enabled
	) VALUES (?::uuid,?::uuid,?::uuid,'CI Key','env',?,'ci-v1',repeat('a',64),100,100,5,TRUE)`,
		f.key, f.tenant, f.channel, f.keyEnv); err != nil {
		return f, err
	}
	if err := execute(`INSERT INTO llm_channel_models(
		tenant_id,channel_id,model_id,upstream_model,priority,weight,enabled
	) VALUES (?::uuid,?::uuid,?::uuid,?,1,100,TRUE)`,
		f.tenant, f.channel, f.model, f.routeAlias); err != nil {
		return f, err
	}
	if err := execute("INSERT INTO llm_task_policies(id,tenant_id,task_key,title,enabled) VALUES (?::uuid,?::uuid,'draft','CI Task',TRUE)", f.policy, f.tenant); err != nil {
		return f, err
	}
	for _, v := range []struct {
		id      string
		version int
		tokens  int
		temp    float64
	}{{f.version1, 1, 128, 0.2}, {f.version2, 2, 64, 0.6}} {
		if err := execute(`INSERT INTO llm_task_policy_versions(
			id,tenant_id,policy_id,version,primary_model_id,max_output_tokens,temperature,
			timeout_seconds,max_retries,required_capabilities,output_mode,task_kinds
		) VALUES (?::uuid,?::uuid,?::uuid,?,?::uuid,?,?,30,0,
			ARRAY['text']::text[],'text',ARRAY['inspiration_analyze']::text[])`,
			v.id, f.tenant, f.policy, v.version, f.model, v.tokens, v.temp); err != nil {
			return f, err
		}
	}
	if err := execute("UPDATE llm_task_policies SET current_version_id=?::uuid WHERE id=?::uuid AND tenant_id=?::uuid",
		f.version1, f.policy, f.tenant); err != nil {
		return f, err
	}
	if err := execute("INSERT INTO agent_definitions(id,tenant_id,definition_key,title,enabled) VALUES (?::uuid,?::uuid,'ci_agent','CI Agent',TRUE)",
		f.definition, f.tenant); err != nil {
		return f, err
	}
	if err := execute(`INSERT INTO agent_definition_versions(
		id,tenant_id,definition_id,version,system_instruction,task_kinds
	) VALUES (?::uuid,?::uuid,?::uuid,1,'CI definition prompt',ARRAY['inspiration_analyze']::text[])`,
		f.defVersion, f.tenant, f.definition); err != nil {
		return f, err
	}
	if err := execute("UPDATE agent_definitions SET current_version_id=?::uuid WHERE id=?::uuid AND tenant_id=?::uuid",
		f.defVersion, f.definition, f.tenant); err != nil {
		return f, err
	}
	return f, nil
}
