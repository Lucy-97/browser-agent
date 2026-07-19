param(
    [string]$Environment = "browser-agent",
    [int]$TimeoutSeconds = 90
)

$ErrorActionPreference = "Stop"
$RootDir = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$EnvFile = Join-Path $RootDir "deploy-local\.env.$Environment"

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { throw "environment file not found: $Path" }
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -notmatch '^([A-Za-z_][A-Za-z0-9_]*)=(.*)$') { continue }
        $value = $matches[2].Trim()
        if ($value.Length -ge 2 -and (($value[0] -eq '"' -and $value[-1] -eq '"') -or ($value[0] -eq "'" -and $value[-1] -eq "'"))) {
            $value = $value.Substring(1, $value.Length - 2)
        }
        [Environment]::SetEnvironmentVariable($matches[1], $value, "Process")
    }
}

function Wait-Http([string]$Uri, [hashtable]$Headers = @{}) {
    $deadline = [DateTime]::UtcNow.AddSeconds(20)
    while ([DateTime]::UtcNow -lt $deadline) {
        try {
            Invoke-WebRequest -UseBasicParsing -Uri $Uri -Headers $Headers -TimeoutSec 2 | Out-Null
            return
        } catch {
            Start-Sleep -Milliseconds 500
        }
    }
    throw "service is not ready: $Uri"
}

function Invoke-Json([string]$Method, [string]$Uri, [hashtable]$Headers, [object]$Body = $null) {
    $params = @{ Method = $Method; Uri = $Uri; Headers = $Headers; UseBasicParsing = $true }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = $Body | ConvertTo-Json -Depth 10 -Compress
    }
    $response = Invoke-WebRequest @params
    return $response.Content | ConvertFrom-Json
}

Import-DotEnv $EnvFile
$webBase = "http://127.0.0.1:$($env:WEB_PORT)/api"
$adminBase = "http://127.0.0.1:$($env:ADMIN_PORT)/api"
$webHeaders = @{}
$adminHeaders = @{}
if ($env:WEB_API_TOKEN) { $webHeaders["X-Web-Token"] = $env:WEB_API_TOKEN }
if ($env:ADMIN_API_TOKEN) { $adminHeaders["X-Admin-Token"] = $env:ADMIN_API_TOKEN }

Wait-Http "$webBase/healthz"
Wait-Http "$adminBase/healthz"

$html = '<form id="f"><input id="search"><button id="submit">Search</button></form><section id="results"></section><script>document.querySelector("#f").addEventListener("submit",e=>{e.preventDefault();document.querySelector("#results").innerHTML="<div class=result>Result: "+document.querySelector("#search").value+"</div>"})</script>'
$fixtureURL = "data:text/html,$([Uri]::EscapeDataString($html))"
$job = Invoke-Json "POST" "$webBase/web/automation/browser-agent-jobs" $webHeaders @{
    url = $fixtureURL
    task = "search the local fixture"
    allowed_domains = @("data:")
    mode = "deterministic_search"
    query = "LiFePO4"
    input_selector = "#search"
    submit_selector = "#submit"
    result_selector = ".result"
    headed = $false
    action_timeout_seconds = 15
}

$deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$jobStatus = $null
while ([DateTime]::UtcNow -lt $deadline) {
    $jobStatus = Invoke-Json "GET" "$adminBase/admin/automation/jobs/$($job.job_id)" $adminHeaders
    if ($jobStatus.status -in @("completed", "failed", "cancelled", "needs_manual_action")) { break }
    Start-Sleep -Seconds 2
}
if ($jobStatus.status -ne "completed") {
    throw "browser job did not complete: job=$($job.job_id) status=$($jobStatus.status) error=$($jobStatus.error_code) $($jobStatus.error_message)"
}

$runs = Invoke-Json "GET" "$webBase/web/automation/jobs/$($job.job_id)/runs" $webHeaders
if (-not $runs.runs -or $runs.runs.Count -ne 1) { throw "expected one run for job $($job.job_id)" }
$run = $runs.runs[0]
$artifactResponse = Invoke-Json "GET" "$adminBase/admin/automation/runs/$($run.run_id)/artifacts" $adminHeaders
$artifactTypes = @($artifactResponse.artifacts | ForEach-Object { $_.artifact_type })
foreach ($requiredType in @("agent_trace", "screenshot")) {
    if ($requiredType -notin $artifactTypes) { throw "missing artifact type: $requiredType" }
}

Write-Output "browser agent Windows E2E passed"
Write-Output "job=$($job.job_id)"
Write-Output "run=$($run.run_id)"
Write-Output "artifacts=$($artifactTypes -join ',')"
