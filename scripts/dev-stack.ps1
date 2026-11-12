#Requires -Version 5.1
<#
.SYNOPSIS
    本地开发基础设施（PostgreSQL 16 + pgvector、Redis 7）的一键生命周期管理。

.DESCRIPTION
    解决两个反复出现的本地环境问题（INK-107）：

    1. Docker 引擎会随调用方进程结束而退出。从脚本或自动化里直接启动 Docker Desktop 时，
       它成为调用方进程树的子进程；调用方一结束，引擎随之消失，容器全部停掉。
       本脚本用 WMI（Win32_Process.Create）或 explorer.exe 启动，让宿主进程不是本脚本，
       从而脱离进程树清理。

    2. 「容器起来了」不等于「能连」。compose 文件为两个服务都定义了 healthcheck，
       本脚本用 up -d --wait 等到真正健康，再执行迁移。

.PARAMETER Action
    Up（默认）：拉起引擎与容器、等待健康、执行数据库迁移。
    Down：停止并删除容器（保留命名卷，数据不丢）。
    Status：打印引擎、容器与库内表数量。

.PARAMETER SkipMigrate
    Up 时跳过迁移（仅起基础设施时用）。

.EXAMPLE
    pwsh -File scripts/dev-stack.ps1 -Action Up
    pwsh -File scripts/dev-stack.ps1 -Action Status
#>
param(
    [ValidateSet('Up', 'Down', 'Status')]
    [string]$Action = 'Up',
    [int]$EngineTimeoutSeconds = 300,
    [switch]$SkipMigrate
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot 'docker-compose.dev.yml'
function Get-DatabaseDsn {
    # DSN 单点定义：迁移与状态查询都跟随仓库根 .env，脚本内不再硬编码主机、用户名、库名或凭据。
    $envFile = Join-Path $repoRoot '.env'
    if (-not (Test-Path $envFile)) {
        throw "未找到 $envFile：请先复制 .env.example 为 .env 并填写 DATABASE_DSN（本脚本不再内置默认连接串）。"
    }
    $line = Get-Content $envFile | Where-Object { $_ -match '^\s*DATABASE_DSN\s*=' } | Select-Object -First 1
    if (-not $line) { throw "未在 $envFile 中找到 DATABASE_DSN。" }
    $value = ($line -replace '^\s*DATABASE_DSN\s*=', '').Trim().Trim([char]'"').Trim([char]39)
    if (-not $value) { throw 'DATABASE_DSN 为空：请填写本地 PostgreSQL 连接串。' }
    return $value
}

function Get-DatabaseIdentity {
    param([Parameter(Mandatory = $true)][string]$Dsn)
    $user = 'ai_novel'
    $name = 'ai_novel'
    $m = [regex]::Match($Dsn, '^postgres(?:ql)?://(?:(?<u>[^:@/]+)(?::[^@]*)?@)?[^/]+/(?<d>[^?]+)')
    if ($m.Success) {
        if ($m.Groups['u'].Success -and $m.Groups['u'].Value) { $user = [System.Uri]::UnescapeDataString($m.Groups['u'].Value) }
        if ($m.Groups['d'].Success -and $m.Groups['d'].Value) { $name = [System.Uri]::UnescapeDataString($m.Groups['d'].Value) }
    }
    return @($user, $name)
}

function Invoke-Migrate {
    # 只有迁移需要 DSN：在这里解析，避免 .env 缺失时连 Down / Status / Up -SkipMigrate 都失败。
    $databaseDSN = Get-DatabaseDsn
    # 注入子进程环境；结束后恢复原有 $env:DATABASE_DSN，避免污染调用方 shell。
    $previousDsn = $env:DATABASE_DSN
    try {
        $env:DATABASE_DSN = $databaseDSN
        & go run ./cmd/migrate -cmd up
        if ($LASTEXITCODE -ne 0) { throw '迁移失败' }
    } finally {
        if ($null -eq $previousDsn) {
            Remove-Item Env:\DATABASE_DSN -ErrorAction SilentlyContinue
        } else {
            $env:DATABASE_DSN = $previousDsn
        }
    }
}


function Test-DockerEngine {
    docker info --format '{{.ServerVersion}}' *> $null
    return $LASTEXITCODE -eq 0
}

function Get-DockerDesktopExe {
    $candidates = @(
        "$env:ProgramFiles\Docker\Docker\Docker Desktop.exe",
        "$env:LOCALAPPDATA\Programs\Docker\Docker\Docker Desktop.exe",
        'E:\AI\DockerDesktop\Docker Desktop.exe'
    )
    foreach ($path in $candidates) {
        if ($path -and (Test-Path $path)) { return $path }
    }
    return $null
}

function Start-DockerEngineDetached {
    # 首选官方 CLI：由 Docker Desktop 自己的守护逻辑托管生命周期。
    docker desktop start *> $null
    if (Test-DockerEngine) { return }

    $exe = Get-DockerDesktopExe
    if (-not $exe) {
        throw '未找到 Docker Desktop.exe，请手工启动 Docker Desktop 后重试。'
    }

    # 兜底一：WMI 创建进程 —— 父进程是 WmiPrvSE，不在本脚本进程树里，不会被连带清理。
    try {
        $null = Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{ CommandLine = '"' + $exe + '"' }
    } catch {
        # 兜底二：explorer.exe 代开，同样脱离本进程树。
        Start-Process -FilePath 'explorer.exe' -ArgumentList ('"' + $exe + '"') | Out-Null
    }
}

function Wait-DockerEngine {
    $deadline = (Get-Date).AddSeconds($EngineTimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Test-DockerEngine) {
            $version = (docker version --format '{{.Server.Version}}').Trim()
            Write-Host "[dev-stack] Docker 引擎就绪（$version）" -ForegroundColor Green
            return
        }
        Start-Sleep -Seconds 5
    }
    throw "等待 Docker 引擎超时（$EngineTimeoutSeconds 秒）。首次启动请手工打开 Docker Desktop 完成初始化。"
}

function Invoke-Compose {
    param([string[]]$ComposeArgs)
    & docker compose -f $composeFile @ComposeArgs
    if ($LASTEXITCODE -ne 0) { throw "docker compose $($ComposeArgs -join ' ') 失败" }
}

function Show-Status {
    Write-Host '[dev-stack] 引擎：' -NoNewline
    if (Test-DockerEngine) {
        Write-Host (docker version --format '{{.Server.Version}}').Trim() -ForegroundColor Green
        docker compose -f $composeFile ps --format '{{.Name}}|{{.Status}}'
        # 表数量查询是可选的：没有 .env 或 DATABASE_DSN 时跳过，不影响容器生命周期管理。
        $statusDsn = $null
        try { $statusDsn = Get-DatabaseDsn } catch { }
        if ($statusDsn) {
            $databaseUser, $databaseName = Get-DatabaseIdentity $statusDsn
            $tables = docker exec inklore-postgres-1 psql -U $databaseUser -d $databaseName -t -A -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'" 2>$null
            if ($LASTEXITCODE -eq 0) { Write-Host "[dev-stack] 库内表数量：$tables" }
        } else {
            Write-Host '[dev-stack] 未配置 .env 的 DATABASE_DSN，跳过库内表数量检查' -ForegroundColor Yellow
        }
    } else {
        Write-Host '未运行' -ForegroundColor Yellow
    }
}

switch ($Action) {
    'Down' {
        if (Test-DockerEngine) { Invoke-Compose @('down') }
        Write-Host '[dev-stack] 容器已停止（命名卷保留，数据不丢）' -ForegroundColor Green
    }
    'Status' { Show-Status }
    'Up' {
        if (-not (Test-DockerEngine)) { Start-DockerEngineDetached }
        Wait-DockerEngine

        Write-Host '[dev-stack] 启动 postgres 与 redis（等待健康检查）...'
        Invoke-Compose @('up', '-d', '--wait')

        if (-not $SkipMigrate) {
            Write-Host '[dev-stack] 执行数据库迁移...'
            Push-Location $repoRoot
            try {
                Invoke-Migrate
                if ($LASTEXITCODE -ne 0) { throw '迁移失败' }
            } finally {
                Pop-Location
            }
        }

        Write-Host ''
        Show-Status
        Write-Host ''
        Write-Host '[dev-stack] 就绪。启动服务示例：' -ForegroundColor Green
        Write-Host '  # DATABASE_DSN 由仓库根 .env 提供（脚本不再打印连接串，也不硬编码凭据）' -ForegroundColor Gray
        Write-Host '  go run ./cmd/server' -ForegroundColor Gray
    }
}
