# PullAI - llama.cpp auto-installer for Windows
# This script downloads and installs llama.cpp into PullAI's bin/ directory

param(
    [string]$InstallDir = "$env:PROGRAMFILES\PullAI"
)

$BinDir = Join-Path $InstallDir "bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

Write-Host "Ricerca ultima versione di llama.cpp..." -ForegroundColor Cyan

try {
    # Get latest release info from GitHub API
    $release = Invoke-RestMethod "https://api.github.com/repos/ggerganov/llama.cpp/releases/latest"
    $tag = $release.tag_name
    Write-Host "Versione: $tag" -ForegroundColor Green

    # Find Windows CPU x64 asset
    $asset = $release.assets | Where-Object { 
        $_.name -match "win" -and $_.name -match "cpu" -and $_.name -match "x64" -and $_.name -match "\.zip$"
    } | Select-Object -First 1

    if (-not $asset) {
        # Fallback: try generic Windows release
        $asset = $release.assets | Where-Object {
            $_.name -match "win.*x64.*zip$" -or $_.name -match "x64.*win.*zip$"
        } | Select-Object -First 1
    }

    if ($asset) {
        $zipPath = Join-Path $env:TEMP "llama-cpp.zip"
        Write-Host "Download: $($asset.name) ($([math]::Round($asset.size/1MB,1)) MB)..." -ForegroundColor Cyan
        
        $wc = New-Object System.Net.WebClient
        $wc.DownloadFile($asset.browser_download_url, $zipPath)
        
        Write-Host "Estrazione in $BinDir..." -ForegroundColor Cyan
        Expand-Archive -LiteralPath $zipPath -DestinationPath $BinDir -Force
        Remove-Item $zipPath -Force

        # Verify llama-cli.exe exists
        $llamaExe = Get-ChildItem -Path $BinDir -Filter "llama-cli.exe" -Recurse | Select-Object -First 1
        if ($llamaExe) {
            # Move to bin root if nested
            if ($llamaExe.Directory.FullName -ne $BinDir) {
                Get-ChildItem -Path $llamaExe.Directory.FullName -Filter "*.exe" | Move-Item -Destination $BinDir -Force
                Get-ChildItem -Path $llamaExe.Directory.FullName -Filter "*.dll" | Move-Item -Destination $BinDir -Force
                Remove-Item $llamaExe.Directory.FullName -Recurse -Force -ErrorAction SilentlyContinue
            }
            Write-Host "llama.cpp installato: $($llamaExe.FullName)" -ForegroundColor Green
        } else {
            Write-Host "ATTENZIONE: llama-cli.exe non trovato nell'archivio." -ForegroundColor Yellow
        }
    } else {
        Write-Host "ERRORE: Nessun asset Windows trovato nella release." -ForegroundColor Red
    }
} catch {
    Write-Host "Errore download: $_" -ForegroundColor Red
    Write-Host "Installa manualmente da: https://github.com/ggerganov/llama.cpp/releases" -ForegroundColor Yellow
}

# Add bin/ to PATH if not already there
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($currentPath -notlike "*$BinDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;$BinDir", "Machine")
    Write-Host "PATH aggiornato: $BinDir aggiunto." -ForegroundColor Green
}

Write-Host "`nDone! Apri un nuovo terminale e prova: llama-cli --version" -ForegroundColor Green
