param(
    [ValidateSet("start", "stop", "restart", "status")]
    [string]$Action = "start",
    [string]$Environment = "browser-agent"
)

$ErrorActionPreference = "Stop"
$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$RunDir = Join-Path $RootDir "deploy-local\run"
$LogDir = Join-Path $RootDir "deploy-local\logs"
$EnvFile = Join-Path $RootDir "deploy-local\.env.$Environment"

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "environment file not found: $Path"
    }
    $values = @{}
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { continue }
        $key = $matches[1]
        $value = $matches[2].Trim()
        if ($value.Length -ge 2 -and (($value[0] -eq '"' -and $value[-1] -eq '"') -or ($value[0] -eq "'" -and $value[-1] -eq "'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        $values[$key] = $value
    }
    foreach ($key in @($values.Keys)) {
        $value = $values[$key]
        $value = [regex]::Replace($value, '\$\{([A-Za-z_][A-Za-z0-9_]*)\}', {
            param($match)
            $name = $match.Groups[1].Value
            if ($values.ContainsKey($name)) { return [string]$values[$name] }
            return [Environment]::GetEnvironmentVariable($name)
        })
        [Environment]::SetEnvironmentVariable($key, $value, "Process")
    }
}

function Stop-ServiceTree([string]$Name) {
    $pidFile = Join-Path $RunDir "$Environment-$Name.pid"
    if (-not (Test-Path -LiteralPath $pidFile)) { return }
    $processId = [int](Get-Content -LiteralPath $pidFile)
    if (Get-Process -Id $processId -ErrorAction SilentlyContinue) {
        & taskkill.exe /PID $processId /T /F | Out-Null
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
}

function Start-Platform {
    Import-DotEnv $EnvFile
    New-Item -ItemType Directory -Force -Path $RunDir, $LogDir | Out-Null

    $artifactDir = $env:ARTIFACT_DIR
    if (-not [IO.Path]::IsPathRooted($artifactDir)) {
        $artifactDir = Join-Path $RootDir $artifactDir
    }
    New-Item -ItemType Directory -Force -Path $artifactDir | Out-Null
    $env:ARTIFACT_DIR = $artifactDir

    $apiPort = $env:API_ADDR.TrimStart(':')
    $env:GO_API_BASE_URL = "http://127.0.0.1:$apiPort"
    $env:ADMIN_API_BASE_URL = $env:GO_API_BASE_URL

    $goCommand = Get-Command go.exe -ErrorAction SilentlyContinue
    if ($goCommand) {
        $go = $goCommand.Source
    } else {
        $go = Join-Path $env:LOCALAPPDATA "CodexTools\go1.25.0\go\bin\go.exe"
        if (-not (Test-Path -LiteralPath $go)) {
            throw "Go runtime not found in PATH or at portable path: $go"
        }
    }
    $apiBinary = Join-Path $RunDir "$Environment-backend-api.exe"
    Push-Location (Join-Path $RootDir "backend-api")
    try {
        & $go build -o $apiBinary ./cmd/api
        if ($LASTEXITCODE -ne 0) { throw "backend-api build failed" }
    } finally {
        Pop-Location
    }

    Stop-ServiceTree "backend-api"
    Stop-ServiceTree "frontend-web"
    Stop-ServiceTree "frontend-admin"

    $api = Start-Process -FilePath $apiBinary -WorkingDirectory (Join-Path $RootDir "backend-api") -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir "$Environment-backend-api.log") `
        -RedirectStandardError (Join-Path $LogDir "$Environment-backend-api.error.log") -PassThru
    Set-Content -LiteralPath (Join-Path $RunDir "$Environment-backend-api.pid") -Value $api.Id

    $web = Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev", "--", "-H", "0.0.0.0", "-p", $env:WEB_PORT) `
        -WorkingDirectory (Join-Path $RootDir "frontend-web") -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir "$Environment-frontend-web.log") `
        -RedirectStandardError (Join-Path $LogDir "$Environment-frontend-web.error.log") -PassThru
    Set-Content -LiteralPath (Join-Path $RunDir "$Environment-frontend-web.pid") -Value $web.Id

    $admin = Start-Process -FilePath "npm.cmd" -ArgumentList @("run", "dev", "--", "-H", "0.0.0.0", "-p", $env:ADMIN_PORT) `
        -WorkingDirectory (Join-Path $RootDir "frontend-admin") -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $LogDir "$Environment-frontend-admin.log") `
        -RedirectStandardError (Join-Path $LogDir "$Environment-frontend-admin.error.log") -PassThru
    Set-Content -LiteralPath (Join-Path $RunDir "$Environment-frontend-admin.pid") -Value $admin.Id

    Write-Output "API:   http://localhost:$apiPort"
    Write-Output "Web:   http://localhost:$($env:WEB_PORT)"
    Write-Output "Admin: http://localhost:$($env:ADMIN_PORT)"
}

function Show-Status {
    Import-DotEnv $EnvFile
    $services = @(
        @{ Name = "API"; Port = [int]$env:API_ADDR.TrimStart(':') },
        @{ Name = "Web"; Port = [int]$env:WEB_PORT },
        @{ Name = "Admin"; Port = [int]$env:ADMIN_PORT }
    )
    foreach ($service in $services) {
        $listener = Get-NetTCPConnection -State Listen -LocalPort $service.Port -ErrorAction SilentlyContinue
        if ($listener) {
            Write-Output "$($service.Name): running on $($service.Port) (pid $($listener.OwningProcess))"
        } else {
            Write-Output "$($service.Name): stopped"
        }
    }
}

switch ($Action) {
    "start" { Start-Platform }
    "stop" {
        Stop-ServiceTree "backend-api"
        Stop-ServiceTree "frontend-web"
        Stop-ServiceTree "frontend-admin"
    }
    "restart" {
        Stop-ServiceTree "backend-api"
        Stop-ServiceTree "frontend-web"
        Stop-ServiceTree "frontend-admin"
        Start-Platform
    }
    "status" { Show-Status }
}
