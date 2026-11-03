<#
.SYNOPSIS
  统计一个进程及其全部子进程（WebView2 等）的内存与空闲 CPU，用于对比 Wails 版与 mygo 原型。

.EXAMPLE
  # 先手动启动应用并停在游戏库页面，等封面加载完、不要动鼠标
  pwsh tools/perf/measure.ps1 -Name desktop-mygo
  pwsh tools/perf/measure.ps1 -Name LunaBox -Seconds 20
#>
param(
  [Parameter(Mandatory = $true)][string]$Name,
  [int]$Seconds = 10
)

$root = Get-Process -Name $Name -ErrorAction SilentlyContinue | Sort-Object StartTime | Select-Object -First 1
if (-not $root) { throw "process '$Name' not found" }

function Get-Tree([int]$rootId) {
  $all = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId, Name
  $ids = [System.Collections.Generic.List[int]]::new()
  $ids.Add($rootId)
  for ($i = 0; $i -lt $ids.Count; $i++) {
    foreach ($p in $all) {
      if ($p.ParentProcessId -eq $ids[$i] -and -not $ids.Contains([int]$p.ProcessId)) { $ids.Add([int]$p.ProcessId) }
    }
  }
  $ids | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue }
}

$tree = Get-Tree $root.Id
$cpuBefore =($tree | ForEach-Object { $_.TotalProcessorTime.TotalMilliseconds } | Measure-Object -Sum).Sum
Start-Sleep -Seconds $Seconds
$tree = Get-Tree $root.Id
$cpuAfter = ($tree | ForEach-Object { $_.TotalProcessorTime.TotalMilliseconds } | Measure-Object -Sum).Sum

$rows = @($tree | ForEach-Object {
  [pscustomobject]@{
    Pid        = $_.Id
    Name       = $_.ProcessName
    WorkingMB  = [math]::Round($_.WorkingSet64 / 1MB, 1)
    PrivateMB  = [math]::Round($_.PrivateMemorySize64 / 1MB, 1)
  }
})
$rows | Format-Table -AutoSize

$ws = ($rows | Measure-Object -Property WorkingMB -Sum).Sum
$priv = ($rows | Measure-Object -Property PrivateMB -Sum).Sum
$cores = [Environment]::ProcessorCount
$cpuPct = ($cpuAfter - $cpuBefore) / ($Seconds * 1000) * 100
"processes: {0}" -f $rows.Count
"total working set: {0:N1} MB" -f $ws
"total private:     {0:N1} MB" -f $priv
"idle cpu over {0}s: {1:N2}% of one core ({2:N2}% of {3} cores)" -f $Seconds, $cpuPct, ($cpuPct / $cores), $cores
