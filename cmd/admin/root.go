package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"InkLore/internal/app"
	"InkLore/internal/platform/config"
	"InkLore/internal/platform/database"
	"InkLore/internal/platform/uow"
	"InkLore/migrations/r1"
)

func runSetRoot() {
	flags := flag.NewFlagSet("set-root", flag.ExitOnError)
	email := flags.String("email", "", "existing operator email to bind as root")
	configPath := flags.String("config", config.DefaultPath, "mounted configuration path")
	_ = flags.Parse(os.Args[2:])
	password := os.Getenv("INKLORE_ROOT_PASSWORD")
	_ = os.Unsetenv("INKLORE_ROOT_PASSWORD")
	if password == "" {
		fail("INKLORE_ROOT_PASSWORD is required")
	}
	cfg, err := config.Bootstrap(*configPath)
	if err != nil {
		fail("invalid R1 configuration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := database.Open(ctx, database.OptionsFromConfig(cfg))
	if err != nil {
		fail("PostgreSQL unavailable")
	}
	defer func() { _ = database.Close(db) }()
	if database.Verify(ctx, db, r1.FS(), r1.Dir) != nil {
		fail("apply and verify R1 migrations before setting root credentials")
	}
	manager, err := uow.New(db)
	if err != nil {
		fail("cannot initialize transaction manager")
	}
	err = app.SetRootCredentials(ctx, manager, *email, password)
	password = ""
	if err != nil {
		fail("root credentials were not updated; verify the operator email, password and migration state")
	}
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		Username string `json:"username"`
	}{Username: "root"})
}
