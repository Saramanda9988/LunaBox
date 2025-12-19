# Important: 请使用interactive SDK PowerShell运行此脚本，确保Playnite API可用
# 中文windows用户必须使用GBK编码保存此脚本以避免乱码问题
# 使用”. .\playnite_exporter.ps1“命令加载脚本
# 指定导出路径
$exportPath = "D:\Playnite_GAL_Export.json"

try {
    Write-Host "--- Start Exporting Game Data ---" -ForegroundColor Cyan
    
    if ($null -eq $PlayniteApi) {
        # 尝试从 Playnite 静态属性获取 API 实例
        try {
            $PlayniteApi = [Playnite.SDK.API]::Instance
        } catch {}
    }

    if ($null -eq $PlayniteApi) {
        throw "Error: Still cannot find Playnite API. Are you sure this is the Playnite SDK window?"
    }

    $allGames = $PlayniteApi.Database.Games
    $exportGames = @()

    foreach ($game in $allGames) {
        $launchPath = ""
        
        # 查找主启动动作 (Type 0 = File)
        $playAction = $game.GameActions | Where-Object { 
            $_.IsPlayAction -eq $true -and ($_.Type -eq 0 -or $_.Type -eq "File")
        } | Select-Object -First 1

        if ($playAction) {
            $expandedPath = $PlayniteApi.ExpandGameVariables($game, $playAction.Path)
            
            if (![System.IO.Path]::IsPathRooted($expandedPath)) {
                $launchPath = [System.IO.Path]::Combine($game.InstallDirectory, $expandedPath)
            } else {
                $launchPath = $expandedPath
            }
        }

        # 构建数据
        $gameData = [PSCustomObject]@{
            id          = $game.Id.ToString()
            name        = $game.Name
            cover_url   = if ($game.CoverImage) { $PlayniteApi.Database.GetFullFilePath($game.CoverImage) } else { $null }
            company     = ($game.Developers -join ", ")
            summary     = $game.Description
            path        = $launchPath
            save_path   = $null
            source_type = "local"
            source_id   = $game.GameId
            cached_at   = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
            created_at  = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
        }

        $exportGames += $gameData
    }

    # 导出并强制使用 UTF8 编码 (PowerShell 5.1 下会带 BOM)
    $exportGames | ConvertTo-Json -Depth 5 | Out-File -FilePath $exportPath -Encoding UTF8
    
    Write-Host "Export Success!" -ForegroundColor Green
    Write-Host "Path: $exportPath"
    Write-Host "Count: $($exportGames.Count)"
}
catch {
    # 修复报错中的乱码显示，移除多余的引号逻辑
    $errMsg = $_.Exception.Message
    Write-Host "ERROR: $errMsg" -ForegroundColor Red
}
