// Command admin performs explicit deployment-only administration.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"InkLore/internal/app"
	llminfra "InkLore/internal/llm/infrastructure"
	"InkLore/internal/platform/config"
	"InkLore/internal/platform/database"
	"InkLore/internal/platform/scope"
	"InkLore/internal/platform/timeutil"
	"InkLore/internal/platform/uow"
	"InkLore/migrations"
)

func main() {
	if len(os.Args) < 2 {
		_, _ = fmt.Fprintln(os.Stderr, "usage:\n  admin bootstrap -tenant-name NAME -email EMAIL -display-name NAME [-username root]\n  admin set-root -email EMAIL [-config PATH]\n  admin seed [-tenant-id ID] [-config PATH]\n  admin seed-tool -tenant-id UUID [-config PATH]")
		os.Exit(2)
	}

	command := os.Args[1]
	switch command {
	case "bootstrap":
		runBootstrap()
	case "seed":
		runSeed()
	case "seed-tool":
		runSeedTool()
	case "set-root":
		runSetRoot()
	default:
		fail("unknown command: " + command)
	}
}

func runSeed() {
	flags := flag.NewFlagSet("seed", flag.ExitOnError)
	tenantID := flags.String("tenant-id", "", "optional tenant id; seeds all tenants if omitted")
	configPath := flags.String("config", config.DefaultPath, "mounted configuration path")
	_ = flags.Parse(os.Args[2:])

	seedConfig, err := loadSeedManifest(defaultSeedManifest)
	if err != nil {
		fail("invalid deployed agent seed manifest: " + err.Error())
	}
	cfg, err := config.Bootstrap(*configPath)
	if err != nil {
		fail("invalid configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := database.Open(ctx, database.OptionsFromConfig(cfg))
	if err != nil {
		fail("PostgreSQL unavailable")
	}
	defer func() { _ = database.Close(db) }()

	var targetTenants []string
	if *tenantID != "" {
		targetTenants = append(targetTenants, *tenantID)
	} else {
		if err := db.WithContext(ctx).Raw("SELECT id FROM platform_tenants").Scan(&targetTenants).Error; err != nil {
			fail("failed to list tenants: " + err.Error())
		}
	}

	var results []*SeedResult
	for _, tid := range targetTenants {
		res, err := seedTenantResources(ctx, db, tid, seedConfig)
		if err != nil {
			fail(fmt.Sprintf("seed tenant %s failed: %v", tid, err))
		}
		results = append(results, res)
	}
	_ = json.NewEncoder(os.Stdout).Encode(results)
}

func runSeedTool() {
	flags := flag.NewFlagSet("seed-tool", flag.ExitOnError)
	tenantID := flags.String("tenant-id", "", "required tenant UUID")
	configPath := flags.String("config", config.DefaultPath, "mounted configuration path")
	_ = flags.Parse(os.Args[2:])
	if flags.NArg() != 0 || !scope.ValidID(*tenantID) {
		fail("seed-tool requires -tenant-id with a valid UUIDv4 and no positional arguments")
	}

	manifest, err := loadSeedManifest(defaultSeedManifest)
	if err != nil {
		fail("invalid deployed agent seed manifest: " + err.Error())
	}
	cfg, err := config.Bootstrap(*configPath)
	if err != nil {
		fail("invalid configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.OptionsFromConfig(cfg))
	if err != nil {
		fail("PostgreSQL unavailable")
	}
	defer func() { _ = database.Close(db) }()

	result, err := seedTenantTool(ctx, db, *tenantID, manifest)
	if err != nil {
		fail("seed-tool failed: " + err.Error())
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func runBootstrap() {
	flags := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	tenant := flags.String("tenant-name", "", "tenant name")
	email := flags.String("email", "", "initial operator email")
	display := flags.String("display-name", "", "initial operator display name")
	username := flags.String("username", "", "reserved Admin username (root)")
	configPath := flags.String("config", config.DefaultPath, "mounted configuration path")
	_ = flags.Parse(os.Args[2:])
	password := os.Getenv("INKLORE_BOOTSTRAP_PASSWORD")
	_ = os.Unsetenv("INKLORE_BOOTSTRAP_PASSWORD")
	if password == "" {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			fail("password input is required")
		}
		password = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	}
	cfg, err := config.Bootstrap(*configPath)
	if err != nil {
		fail("invalid configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.OptionsFromConfig(cfg))
	if err != nil {
		fail("PostgreSQL unavailable")
	}
	defer func() { _ = database.Close(db) }()
	if database.Verify(ctx, db, migrations.FS(), migrations.Dir) != nil {
		fail("apply and verify migrations before bootstrap")
	}
	manager, err := uow.New(db)
	if err != nil {
		fail("cannot initialize transaction manager")
	}
	budgetRepo := llminfra.NewBudgetRepository(manager)
	result, err := app.BootstrapIdentityWithUserProvision(ctx, manager, *tenant, *email, *username, *display, password, func(tx context.Context, tenantID string) error {
		return budgetRepo.Provision(tx, tenantID, timeutil.Now())
	})
	password = ""
	if err != nil {
		fail("bootstrap refused; verify identity fields and budget provisioning")
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
}
func fail(message string) {
	_, _ = fmt.Fprintln(os.Stderr, "admin:", message)
	os.Exit(1)
}
