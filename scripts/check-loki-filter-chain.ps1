param(
  [Parameter(Mandatory = $true)]
  [string]$GrafanaUrl,

  [Parameter(Mandatory = $true)]
  [string]$GrafanaToken,

  [string]$DatasourceUid = "loki",
  [string]$Namespace = "e-team",
  [string]$Container = "execution-service",
  [string]$Pipeline = "Calctl Public Pipeline",
  [string]$FromMs = "",
  [string]$ToMs = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($FromMs) -or [string]::IsNullOrWhiteSpace($ToMs)) {
  $to = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
  $from = $to - 3600000
  if ([string]::IsNullOrWhiteSpace($FromMs)) { $FromMs = "$from" }
  if ([string]::IsNullOrWhiteSpace($ToMs)) { $ToMs = "$to" }
}

$selector = "{namespace=`"$Namespace`", container=`"$Container`"}"

$tests = @(
  @{ Name = "base"; Expr = $selector },
  @{ Name = "json"; Expr = "$selector | json" },
  @{ Name = "event"; Expr = "$selector | json | event=`"pipeline-run`"" },
  @{ Name = "status"; Expr = "$selector | json | event=`"pipeline-run`" | status=~`"success|failed`"" },
  @{ Name = "pipeline"; Expr = "$selector | json | event=`"pipeline-run`" | status=~`"success|failed`" | pipeline=`"$Pipeline`"" },
  @{ Name = "linefmt+json"; Expr = "$selector | json | line_format `"{{.log}}`" | json | event=`"pipeline-run`" | status=~`"success|failed`" | pipeline=`"$Pipeline`"" }
)

function Get-StatValue {
  param(
    [object[]]$Stats,
    [string]$DisplayName
  )
  $item = $Stats | Where-Object { $_.displayName -eq $DisplayName } | Select-Object -First 1
  if ($null -eq $item) { return "-" }
  return "$($item.value)"
}

function Get-FrameRowCount {
  param([object]$Frame)
  if ($null -eq $Frame -or $null -eq $Frame.data -or $null -eq $Frame.data.values) { return 0 }
  foreach ($fieldValues in $Frame.data.values) {
    if ($null -ne $fieldValues -and $fieldValues.Count -gt 0) {
      return $fieldValues.Count
    }
  }
  return 0
}

Write-Host "Grafana URL : $GrafanaUrl"
Write-Host "Datasource  : $DatasourceUid"
Write-Host "Range(ms)   : $FromMs -> $ToMs"
Write-Host ("Range(UTC)  : {0} -> {1}" -f `
  [DateTimeOffset]::FromUnixTimeMilliseconds([int64]$FromMs).UtcDateTime.ToString("yyyy-MM-dd HH:mm:ss"), `
  [DateTimeOffset]::FromUnixTimeMilliseconds([int64]$ToMs).UtcDateTime.ToString("yyyy-MM-dd HH:mm:ss"))
Write-Host ""
Write-Host "step            rows  processed  sent"
Write-Host "--------------  ----  ---------  ----"

foreach ($t in $tests) {
  $body = @{
    queries = @(
      @{
        refId = "A"
        datasource = @{
          type = "loki"
          uid = $DatasourceUid
        }
        expr = $t.Expr
        queryType = "range"
        maxDataPoints = 655
        intervalMs = 5000
      }
    )
    from = "$FromMs"
    to = "$ToMs"
  } | ConvertTo-Json -Depth 15

  $responseText = curl.exe -sS -X POST "$GrafanaUrl/api/ds/query" `
    -H "Authorization: Bearer $GrafanaToken" `
    -H "Content-Type: application/json" `
    --data-raw $body

  $resp = $responseText | ConvertFrom-Json
  $frame = $resp.results.A.frames | Select-Object -First 1
  $rows = Get-FrameRowCount -Frame $frame
  $stats = $frame.schema.meta.stats
  $processed = Get-StatValue -Stats $stats -DisplayName "Summary: total lines processed"
  $sent = Get-StatValue -Stats $stats -DisplayName "Ingester: total lines sent"

  "{0,-14}  {1,4}  {2,9}  {3,4}" -f $t.Name, $rows, $processed, $sent
}

Write-Host ""
Write-Host "How to read:"
Write-Host "- rows > 0 means this step returns data."
Write-Host "- If a step drops from rows>0 to rows=0, the condition added in that step filtered everything."
Write-Host "- processed > 0 and sent = 0 means logs were scanned but fully filtered out."
