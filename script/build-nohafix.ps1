$ErrorActionPreference = 'Stop'

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot '..')
$buildCountFile = Join-Path $repoRoot 'build_count.txt'

if (Test-Path $buildCountFile) {
    $rawBuildCount = (Get-Content -Path $buildCountFile -Raw).Trim().Trim([char]0xFEFF)
    if ($rawBuildCount -match '^\d+$') {
        $buildCount = [int]$rawBuildCount
    } else {
        $buildCount = 0
    }
} else {
    $buildCount = 0
}

$buildCount++
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText($buildCountFile, "$buildCount`n", $utf8NoBom)

$env:GOSUMDB = 'off'
$env:GOPROXY = 'https://goproxy.cn,direct'
$env:CGO_ENABLED = '0'

function Build-Meow {
    param(
        [string]$Goos,
        [string]$Goarch,
        [string]$Output,
        [string]$Goarm = ''
    )

    $env:GOOS = $Goos
    $env:GOARCH = $Goarch
    if ($Goarm -ne '') {
        $env:GOARM = $Goarm
    } else {
        Remove-Item Env:\GOARM -ErrorAction SilentlyContinue
    }

    Write-Host "Building $Output with version 1.5-nohafix$buildCount"
    & go build -trimpath -ldflags "-s -w -X main.nohaFixBuild=$buildCount" -o $Output .
}

Push-Location $repoRoot
try {
    Build-Meow -Goos 'windows' -Goarch 'amd64' -Output 'meow-windows-amd64.exe'
    Build-Meow -Goos 'linux' -Goarch 'arm' -Goarm '7' -Output 'meow-linux-armv7'
    Build-Meow -Goos 'linux' -Goarch 'arm64' -Output 'meow-linux-arm64'
} finally {
    Pop-Location
}

Write-Host "Build complete. Current version: 1.5-nohafix$buildCount"
