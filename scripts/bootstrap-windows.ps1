param(
    [string]$BinDir
)

$ErrorActionPreference = 'Stop'

function Fail-Wsp {
    param([string]$Reason)
    [Console]::Error.WriteLine('STATUS: BLOCKED')
    [Console]::Error.WriteLine("REASON: $Reason")
    exit 2
}

function Test-PathEntry {
    param([string]$PathValue, [string]$Expected)
    if ([string]::IsNullOrWhiteSpace($PathValue)) { return $false }
    $expectedExpanded = [Environment]::ExpandEnvironmentVariables($Expected.Trim().Trim('"')).TrimEnd('\','/')
    foreach ($part in ($PathValue -split ';')) {
        if ([string]::IsNullOrWhiteSpace($part)) { continue }
        $candidate = [Environment]::ExpandEnvironmentVariables($part.Trim().Trim('"')).TrimEnd('\','/')
        if ([string]::Equals($candidate, $expectedExpanded, [System.StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
    }
    return $false
}

function Test-ProductLauncher {
    param([string]$PathValue)
    if ([string]::IsNullOrWhiteSpace($PathValue) -or -not (Test-Path -LiteralPath $PathValue -PathType Leaf)) { return $false }
    try {
        $firstLines = @(Get-Content -LiteralPath $PathValue -TotalCount 2 -ErrorAction Stop)
    } catch {
        return $false
    }
    return ($firstLines.Count -ge 2 -and $firstLines[0] -eq '@echo off' -and $firstLines[1] -eq 'REM WSP_PRODUCT_LAUNCHER_V1')
}

function Escape-BatchLiteral {
    param([string]$Value)
    return $Value.Replace('%', '%%')
}

if ($env:OS -ne 'Windows_NT') {
    Fail-Wsp 'WINDOWS_BOOTSTRAP_REQUIRES_WINDOWS'
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$productRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptDir '..'))
$adapterPath = [System.IO.Path]::GetFullPath((Join-Path $scriptDir 'wsp-windows.ps1'))
if (-not (Test-Path -LiteralPath $adapterPath -PathType Leaf)) { Fail-Wsp 'WINDOWS_ADAPTER_NOT_FOUND' }

$shellCommand = Get-Command pwsh.exe -ErrorAction SilentlyContinue
if (-not $shellCommand) { $shellCommand = Get-Command powershell.exe -ErrorAction SilentlyContinue }
if (-not $shellCommand -or [string]::IsNullOrWhiteSpace($shellCommand.Source)) {
    Fail-Wsp 'POWERSHELL_RUNTIME_NOT_FOUND'
}
$shellPath = [System.IO.Path]::GetFullPath($shellCommand.Source)

$existing = Get-Command wsp -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
$explicitBinDir = -not [string]::IsNullOrWhiteSpace($BinDir)
if (-not $explicitBinDir -and $existing -and (Test-ProductLauncher $existing.Source)) {
    $BinDir = Split-Path -Parent $existing.Source
}
if ([string]::IsNullOrWhiteSpace($BinDir)) {
    $BinDir = Join-Path $HOME '.local\bin'
}
$BinDir = [System.IO.Path]::GetFullPath($BinDir)
$target = Join-Path $BinDir 'wsp.cmd'

if ($existing -and -not [string]::IsNullOrWhiteSpace($existing.Source)) {
    $existingPath = [System.IO.Path]::GetFullPath($existing.Source)
    if (-not [string]::Equals($existingPath, $target, [System.StringComparison]::OrdinalIgnoreCase)) {
        if (Test-ProductLauncher $existingPath) {
            if ($explicitBinDir) { Fail-Wsp "EXISTING_PRODUCT_WSP_DIFFERENT_TARGET:$existingPath" }
            $BinDir = Split-Path -Parent $existingPath
            $target = $existingPath
        } else {
            Fail-Wsp "UNRELATED_WSP_COMMAND_EXISTS:$existingPath"
        }
    } elseif (-not (Test-ProductLauncher $existingPath)) {
        Fail-Wsp "UNRELATED_WSP_COMMAND_EXISTS:$existingPath"
    }
}

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
if (-not (Test-Path -LiteralPath $BinDir -PathType Container)) { Fail-Wsp "BIN_DIR_NOT_DIRECTORY:$BinDir" }

if ((Test-Path -LiteralPath $target) -and -not (Test-Path -LiteralPath $target -PathType Leaf)) {
    Fail-Wsp "TARGET_WSP_NOT_REGULAR_FILE:$target"
}
if ((Test-Path -LiteralPath $target -PathType Leaf) -and -not (Test-ProductLauncher $target)) {
    Fail-Wsp "TARGET_WSP_EXISTS:$target"
}

$batchShell = Escape-BatchLiteral $shellPath
$batchAdapter = Escape-BatchLiteral $adapterPath
$launcherContent = @(
    '@echo off',
    'REM WSP_PRODUCT_LAUNCHER_V1',
    "REM WSP_PRODUCT_ADAPTER=$batchAdapter",
    "\"$batchShell\" -NoProfile -ExecutionPolicy Bypass -File \"$batchAdapter\" %*",
    'exit /b %ERRORLEVEL%'
) -join "`r`n"
$launcherContent += "`r`n"

$result = 'REGISTERED'
if (Test-Path -LiteralPath $target -PathType Leaf) {
    $currentContent = [System.IO.File]::ReadAllText($target)
    if ($currentContent -eq $launcherContent) {
        $result = 'NO_CHANGE_REQUIRED'
    } else {
        [System.IO.File]::WriteAllText($target, $launcherContent, [System.Text.Encoding]::ASCII)
        $result = 'UPDATED_PRODUCT_LAUNCHER'
    }
} else {
    [System.IO.File]::WriteAllText($target, $launcherContent, [System.Text.Encoding]::ASCII)
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$pathResult = 'ALREADY_PRESENT'
if (-not (Test-PathEntry $userPath $BinDir)) {
    $newUserPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $BinDir } else { "$userPath;$BinDir" }
    [Environment]::SetEnvironmentVariable('Path', $newUserPath, 'User')
    $pathResult = 'ADDED'
}

if (-not (Test-PathEntry $env:Path $BinDir)) {
    $env:Path = "$BinDir;$env:Path"
    $processPathResult = 'UPDATED_CHILD_PROCESS'
} else {
    $processPathResult = 'ALREADY_PRESENT'
}

$oldPreference = $ErrorActionPreference
$ErrorActionPreference = 'Continue'
try {
    $versionOutput = & $target version 2>&1
    $versionCode = $LASTEXITCODE
    & cmd.exe /d /s /c "where wsp >nul 2>&1 && wsp version >nul 2>&1"
    $cmdCode = $LASTEXITCODE
    $verifyCommand = '$c = Get-Command wsp -CommandType Application -ErrorAction SilentlyContinue; if (-not $c) { exit 41 }; & wsp version | Out-Null; exit $LASTEXITCODE'
    & $shellPath -NoProfile -ExecutionPolicy Bypass -Command $verifyCommand
    $powershellCode = $LASTEXITCODE
} finally {
    $ErrorActionPreference = $oldPreference
}

if ($versionCode -ne 0 -or (($versionOutput | Out-String) -notmatch 'CLI: wsp')) {
    Fail-Wsp 'WINDOWS_LAUNCHER_VERIFICATION_FAILED'
}
if ($cmdCode -ne 0) { Fail-Wsp 'CMD_COMMAND_RESOLUTION_FAILED' }
if ($powershellCode -ne 0) { Fail-Wsp 'POWERSHELL_COMMAND_RESOLUTION_FAILED' }

Write-Output 'WORKSPACE OPS WINDOWS BOOTSTRAP'
Write-Output "RESULT: $result"
Write-Output "COMMAND_PATH: $target"
Write-Output "SOURCE_ADAPTER: $adapterPath"
Write-Output "USER_PATH_ENTRY: $BinDir"
Write-Output "PATH_REGISTRATION: $pathResult"
Write-Output "CURRENT_PROCESS_PATH: $processPathResult"
Write-Output 'CMD_SURFACE: PASS'
Write-Output 'POWERSHELL_SURFACE: PASS'
Write-Output 'ALIAS_REQUIRED: NO'
Write-Output 'PLATFORM_SUPPORT: IMPLEMENTED_CI_VERIFIED'
Write-Output 'PHYSICAL_WINDOWS_ACCEPTANCE: PENDING'
Write-Output 'STATUS: PASS'
exit 0
