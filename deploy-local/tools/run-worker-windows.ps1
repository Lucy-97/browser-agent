param(
    [ValidateSet("init", "pair", "doctor", "start", "stop", "restart", "status")]
    [string]$Action = "status",
    [string]$Environment = "browser-agent"
)

$ErrorActionPreference = "Stop"
$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$RunDir = Join-Path $RootDir "deploy-local\run"
$LogDir = Join-Path $RootDir "deploy-local\logs"
$EnvFile = Join-Path $RootDir "deploy-local\.env.$Environment"
$WorkerDir = Join-Path $RootDir "worker\local-cli"
$WorkerExecutable = Join-Path $WorkerDir ".venv\Scripts\qiyuan-worker.exe"
$PidFile = Join-Path $RunDir "$Environment-worker.pid"

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { throw "environment file not found: $Path" }
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
        [Environment]::SetEnvironmentVariable($key, [string]$values[$key], "Process")
    }
    $env:QIYUAN_ENV = $Environment
}

function Invoke-Worker([string[]]$Arguments) {
    & $WorkerExecutable @Arguments
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Stop-Worker {
    if (-not (Test-Path -LiteralPath $PidFile)) { return }
    $processId = [int](Get-Content -LiteralPath $PidFile)
    if (Get-Process -Id $processId -ErrorAction SilentlyContinue) {
        & taskkill.exe /PID $processId /T /F | Out-Null
    }
    Remove-Item -LiteralPath $PidFile -Force -ErrorAction SilentlyContinue
}

Import-DotEnv $EnvFile
if (-not (Test-Path -LiteralPath $WorkerExecutable)) { throw "worker executable missing: $WorkerExecutable" }
New-Item -ItemType Directory -Force -Path $RunDir, $LogDir | Out-Null

switch ($Action) {
    "init" { Invoke-Worker @("init", "--server", $env:WORKER_SERVER_URL) }
    "pair" { Invoke-Worker @("pair", "--display-name", $env:WORKER_DISPLAY_NAME, "--timeout-seconds", "60") }
    "doctor" { Invoke-Worker @("doctor") }
    "start" {
        Stop-Worker
        $process = Start-Process -FilePath $WorkerExecutable -ArgumentList @("run") `
            -WorkingDirectory $WorkerDir -WindowStyle Hidden `
            -RedirectStandardOutput (Join-Path $LogDir "$Environment-worker.log") `
            -RedirectStandardError (Join-Path $LogDir "$Environment-worker.error.log") -PassThru
        Set-Content -LiteralPath $PidFile -Value $process.Id
        Write-Output "Worker started (pid $($process.Id))"
    }
    "stop" { Stop-Worker }
    "restart" {
        Stop-Worker
        $process = Start-Process -FilePath $WorkerExecutable -ArgumentList @("run") `
            -WorkingDirectory $WorkerDir -WindowStyle Hidden `
            -RedirectStandardOutput (Join-Path $LogDir "$Environment-worker.log") `
            -RedirectStandardError (Join-Path $LogDir "$Environment-worker.error.log") -PassThru
        Set-Content -LiteralPath $PidFile -Value $process.Id
        Write-Output "Worker restarted (pid $($process.Id))"
    }
    "status" {
        Invoke-Worker @("status")
        if (Test-Path -LiteralPath $PidFile) {
            $processId = [int](Get-Content -LiteralPath $PidFile)
            if (Get-Process -Id $processId -ErrorAction SilentlyContinue) {
                Write-Output "process: running (pid $processId)"
            } else {
                Write-Output "process: stopped"
            }
        } else {
            Write-Output "process: stopped"
        }
    }
}
