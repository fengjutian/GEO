# GeoPilot live API smoke validation
#
# Prerequisites:
#   - GeoPilot API is running.
#   - The database behind it is a disposable test database.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File scripts/smoke-api.ps1
#   powershell ... -ApiBase http://127.0.0.1:8080/api -Target https://example.org -FailureTarget https://expired.badssl.com
#
# The script creates a temporary site, scans it twice, verifies issue status
# lifecycle/comparison/reports/dashboard, then deletes the site.

param(
    [string]$ApiBase = 'http://127.0.0.1:8080/api',
    [string]$Target = 'https://example.org',
    [string]$FailureTarget = ''
)

$ErrorActionPreference = 'Stop'

function Invoke-ApiJson {
    param(
        [string]$Method,
        [string]$Path,
        $Body
    )
    $params = @{
        Uri         = "$ApiBase$Path"
        Method      = $Method
        TimeoutSec  = 90
    }
    if ($null -ne $Body) {
        $json = $Body | ConvertTo-Json -Compress
        $params.ContentType = 'application/json; charset=utf-8'
        $params.Body = [Text.Encoding]::UTF8.GetBytes($json)
    }
    Invoke-RestMethod @params
}

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw "ASSERTION FAILED: $Message" }
    Write-Host "  ok  $Message"
}

function Wait-Scan {
    param([int]$ScanID, [int]$TimeoutSeconds = 90)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $detail = Invoke-ApiJson GET "/scans/$ScanID" $null
        if ($detail.scan.status -in @('completed', 'failed', 'cancelled')) {
            return $detail
        }
        Start-Sleep -Milliseconds 800
    }
    throw "ASSERTION FAILED: scan $ScanID did not finish in time"
}

$createdSite = $null
$createdFailureSite = $null
try {
    Write-Host "GeoPilot smoke: $ApiBase -> $Target"
    $health = Invoke-ApiJson GET '/health' $null
    Assert-True ($health.status -eq 'ok') 'health endpoint returns ok'

    $createdSite = Invoke-ApiJson POST '/sites' @{
        name        = "Smoke $([DateTime]::Now.ToString('yyyyMMdd-HHmmss'))"
        url         = $Target
        description = 'Automated smoke validation'
    }
    Assert-True ($createdSite.id -gt 0) 'site created'

    $firstQueued = Invoke-ApiJson POST "/sites/$($createdSite.id)/scans" @{}
    Assert-True ($firstQueued.scan.status -in @('queued', 'running', 'completed')) 'scan job accepted'
    $first = Wait-Scan -ScanID $firstQueued.scan.id
    Assert-True ($first.scan.status -eq 'completed') 'first scan completed'
    Assert-True ($first.findings.Count -gt 0) 'first scan produced findings'
    Assert-True (-not [string]::IsNullOrWhiteSpace($first.scan.finalUrl)) 'final URL persisted'

    $findings = Invoke-ApiJson GET "/sites/$($createdSite.id)/findings" $null
    $issue = $findings | Where-Object { $_.ruleCode -eq 'CANONICAL_MISSING' } | Select-Object -First 1
    if (-not $issue) { $issue = $findings[0] }

    $updated = Invoke-ApiJson PATCH "/issues/$($issue.id)" @{ status = 'resolved'; note = 'smoke test' }
    Assert-True ($updated.status -eq 'resolved') 'issue marked resolved'

    $secondQueued = Invoke-ApiJson POST "/sites/$($createdSite.id)/scans" @{}
    $second = Wait-Scan -ScanID $secondQueued.scan.id
    $reopened = $second.findings | Where-Object { $_.ruleCode -eq $issue.ruleCode } | Select-Object -First 1
    Assert-True ($reopened.status -eq 'open') 'recurring issue reopened automatically'

    $detail = Invoke-ApiJson GET "/scans/$($second.scan.id)" $null
    Assert-True ($detail.comparison.baseScanId -eq $first.scan.id) 'scan comparison uses previous successful scan'

    $html = Invoke-WebRequest "$ApiBase/scans/$($second.scan.id)/report" -UseBasicParsing
    Assert-True ($html.StatusCode -eq 200 -and $html.Content.Length -gt 1000) 'HTML report is available'
    $jsonReport = Invoke-ApiJson GET "/scans/$($second.scan.id)/report.json" $null
    Assert-True ($jsonReport.formatVersion -eq '1.0') 'JSON report is available'

    $dashboard = Invoke-ApiJson GET '/dashboard' $null
    Assert-True ($dashboard.stats.siteCount -ge 1) 'dashboard includes the site'
    Assert-True ($dashboard.recentScans.Count -ge 2) 'dashboard includes recent scans'

    if ($FailureTarget) {
        $createdFailureSite = Invoke-ApiJson POST '/sites' @{
            name        = "Smoke failure $([DateTime]::Now.ToString('yyyyMMdd-HHmmss'))"
            url         = $FailureTarget
            description = 'Failure-path smoke validation'
        }
        $failureQueued = Invoke-ApiJson POST "/sites/$($createdFailureSite.id)/scans" @{}
        $failure = Wait-Scan -ScanID $failureQueued.scan.id
        Assert-True ($failure.scan.status -eq 'failed') 'failure target records failed status'
        Assert-True (-not [string]::IsNullOrWhiteSpace($failure.scan.errorCode)) 'failure code persisted'
    }

    Write-Host 'SMOKE PASSED'
} finally {
    if ($createdFailureSite -and $createdFailureSite.id) {
        try { Invoke-ApiJson DELETE "/sites/$($createdFailureSite.id)" $null | Out-Null } catch { Write-Warning "cleanup failure site: $($_.Exception.Message)" }
    }
    if ($createdSite -and $createdSite.id) {
        try { Invoke-ApiJson DELETE "/sites/$($createdSite.id)" $null | Out-Null } catch { Write-Warning "cleanup site: $($_.Exception.Message)" }
    }
}
