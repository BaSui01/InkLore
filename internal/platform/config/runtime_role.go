package config

import (
	"fmt"
	"os"
	"strings"
)

const RuntimeRoleEnv = "APP_RUNTIME_ROLE"

type RuntimeRole string

const (
	RuntimeRoleAll      RuntimeRole = "all"
	RuntimeRoleServer   RuntimeRole = "server"
	RuntimeRoleExecutor RuntimeRole = "executor"
)

func ResolveRuntimeRole() (RuntimeRole, error) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(RuntimeRoleEnv)))
	if raw == "" {
		return RuntimeRoleAll, nil
	}
	role := RuntimeRole(raw)
	if !role.Valid() {
		return "", fmt.Errorf("%s must be one of all, server, executor, got %q", RuntimeRoleEnv, raw)
	}
	return role, nil
}

func (r RuntimeRole) Valid() bool {
	switch r {
	case RuntimeRoleAll, RuntimeRoleServer, RuntimeRoleExecutor:
		return true
	default:
		return false
	}
}

func (r RuntimeRole) ServesHTTP() bool {
	return r == RuntimeRoleAll || r == RuntimeRoleServer
}

func (r RuntimeRole) RunsExecutor() bool {
	return r == RuntimeRoleAll || r == RuntimeRoleExecutor
}

func (r RuntimeRole) RunsSchedulers() bool {
	return r == RuntimeRoleAll || r == RuntimeRoleServer
}

func (r RuntimeRole) RequiresSharedQueue() bool {
	return r == RuntimeRoleServer || r == RuntimeRoleExecutor
}
