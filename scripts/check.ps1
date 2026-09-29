<#
.SYNOPSIS
    InkLore 统一校验脚本（Windows / PowerShell 版），等价于 make lint。

.DESCRIPTION
    顺序执行：gofmt 格式校验 → go build → go vet → archguard 架构红线。
    任一环节失败则以退出码 1 结束，便于本地与 CI 复用同一套判据。

.PARAMETER Reachability
    额外输出运行路径可达性报告（哪些包不在 cmd/server 依赖图中）。

.PARAMETER IncludeFrontend
    额外执行前端类型检查与构建（npm run build）。

.EXAMPLE
    pwsh scripts/check.ps1
    pwsh scripts/check.ps1 -Reachability -IncludeFrontend
#>
[CmdletBinding()]
param(
    [switch]$Reachability,
    [switch]$IncludeFrontend
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$script:failed = @()

function Test-Step {
    param(
        [string]$Name,
        [scriptblock]$Body
    )
    Write-Host ""
    Write-Host "=== $Name ===" -ForegroundColor Cyan
    $global:LASTEXITCODE = 0
    & $Body
    if ($LASTEXITCODE -ne 0) {
        $script:failed += $Name
        Write-Host "  x 失败" -ForegroundColor Red
    }
    else {
        Write-Host "  v 通过" -ForegroundColor Green
    }
}

Test-Step 'gofmt 格式校验' {
    $unformatted = @(gofmt -l . 2>&1 | Where-Object { $_ -ne '' })
    if ($unformatted.Count -gt 0) {
        Write-Host '以下文件未格式化，请运行 gofmt -w . ：' -ForegroundColor Red
        $unformatted | ForEach-Object { Write-Host "    $_" }
        $global:LASTEXITCODE = 1
    }
}

Test-Step 'go build ./...' { go build ./... }

Test-Step 'go vet ./...' { go vet ./... }

Test-Step '前端 i18n key 完整性' {
    go run ./cmd/i18ncheck frontend/packages/i18n/src/locales/zh-CN.ts frontend/packages/i18n/src/locales/en-US.ts
}

Test-Step 'archguard 架构红线校验' {
    if ($Reachability) {
        go run ./cmd/archguard -reachability
    }
    else {
        go run ./cmd/archguard
    }
}

if ($IncludeFrontend) {
    Test-Step '前端类型检查与构建' {
        Push-Location (Join-Path $root 'frontend')
        try { npm run build } finally { Pop-Location }
    }
}

Write-Host ""
if ($script:failed.Count -gt 0) {
    Write-Host ("校验失败：" + ($script:failed -join '、')) -ForegroundColor Red
    exit 1
}
Write-Host '全部校验通过' -ForegroundColor Green
exit 0
