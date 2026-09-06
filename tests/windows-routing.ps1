$ErrorActionPreference = 'Stop'

$Pass = 0
$Fail = 0

function Ok {
    param([string]$Label)
    $script:Pass++
    Write-Output "PASS: $Label"
}

function Fail-Test {
    param([string]$Label, [string]$Detail = '')
    $script:Fail++
    [Console]::Error.WriteLine("FAIL: $Label")
    if (-not [string]::IsNullOrWhiteSpace($Detail)) { [Console]::Error.WriteLine($Detail) }
}

function Contains {
    param([string]$Text, [string]$Needle, [string]$Label)
    if ($Text.Contains($Needle)) { Ok $Label } else { Fail-Test $Label $Text }
}

function Test-PathEntry {
    param([string]$PathValue, [string]$Expected)
    if ([string]::IsNullOrWhiteSpace($PathValue)) { return $false }
    $expectedExpanded = [Environment]::ExpandEnvironmentVariables($Expected.Trim().Trim('"')).TrimEnd('\','/')
    foreach ($part in ($PathValue -split ';')) {
        if ([string]::IsNullOrWhiteSpace($part)) { continue }
        $candidate = [Environment]::ExpandEnvironmentVariables($part.Trim().Trim('"')).TrimEnd('\','/')
        if ([string]::Equals($candidate, $expectedExpanded, [System.StringComparison]::OrdinalIgnoreCase)) { return $true }
    }
    return $false
}

function Count-PathEntry {
    param([string]$PathValue, [string]$Expected)
    if ([string]::IsNullOrWhiteSpace($PathValue)) { return 0 }
    $expectedExpanded = [Environment]::ExpandEnvironmentVariables($Expected.Trim().Trim('"')).TrimEnd('\','/')
    $count = 0
    foreach ($part in ($PathValue -split ';')) {
        if ([string]::IsNullOrWhiteSpace($part)) { continue }
        $candidate = [Environment]::ExpandEnvironmentVariables($part.Trim().Trim('"')).TrimEnd('\','/')
        if ([string]::Equals($candidate, $expectedExpanded, [System.StringComparison]::OrdinalIgnoreCase)) { $count++ }
    }
    return $count
}

function Invoke-PowerShellFile {
    param([string]$File, [string[]]$FileArgs)
    $ps = Get-Command pwsh.exe -ErrorAction SilentlyContinue
    if (-not $ps) { $ps = Get-Command powershell.exe -ErrorAction SilentlyContinue }
    if (-not $ps) { throw 'PowerShell executable not found for test fixture.' }
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $ps.Source -NoProfile -ExecutionPolicy Bypass -File $File @FileArgs 2>&1
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    return [pscustomobject]@{ Output = (($output | ForEach-Object { $_.ToString() }) -join "`n"); Code = $code }
}

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$SourceAdapter = Join-Path $Root 'scripts\wsp-windows.ps1'
$SourceBootstrap = Join-Path $Root 'scripts\bootstrap-windows.ps1'
$SourceCore = Join-Path $Root 'bin\wsp'

if ($env:OS -ne 'Windows_NT') {
    Write-Output 'SKIP: Windows native routing acceptance requires Windows'
    exit 0
}

if ((Test-Path -LiteralPath $SourceAdapter -PathType Leaf) -and
    (Test-Path -LiteralPath $SourceBootstrap -PathType Leaf) -and
    (Test-Path -LiteralPath $SourceCore -PathType Leaf)) {
    Ok 'Windows Product routing files present'
} else {
    Fail-Test 'Windows Product routing files present'
}

$Temp = Join-Path ([System.IO.Path]::GetTempPath()) ("wsp p0d3 r1 " + [guid]::NewGuid().ToString('N'))
$Product = Join-Path $Temp 'product root with spaces'
$BinDir = Join-Path $Temp 'user bin with spaces'
$CollisionDir = Join-Path $Temp 'collision bin'
$Workspace = Join-Path $Temp 'workspace root with spaces'
$StateHome = Join-Path $Temp 'state root with spaces'

$SavedUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$SavedProcessPath = $env:Path
$SavedStateHome = $env:WSP_STATE_HOME

try {
    New-Item -ItemType Directory -Force -Path (Join-Path $Product 'bin') | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $Product 'scripts') | Out-Null
    New-Item -ItemType Directory -Force -Path $CollisionDir | Out-Null
    New-Item -ItemType Directory -Force -Path $Workspace | Out-Null

    Copy-Item -LiteralPath $SourceCore -Destination (Join-Path $Product 'bin\wsp')
    Copy-Item -LiteralPath $SourceAdapter -Destination (Join-Path $Product 'scripts\wsp-windows.ps1')
    Copy-Item -LiteralPath $SourceBootstrap -Destination (Join-Path $Product 'scripts\bootstrap-windows.ps1')
    Copy-Item -LiteralPath (Join-Path $Root 'VERSION') -Destination (Join-Path $Product 'VERSION')

    $Bootstrap = Join-Path $Product 'scripts\bootstrap-windows.ps1'
    $Target = Join-Path $BinDir 'wsp.cmd'

    $collisionLauncher = Join-Path $CollisionDir 'wsp.cmd'
    [System.IO.File]::WriteAllText($collisionLauncher, "@echo off`r`nexit /b 9`r`n", [System.Text.Encoding]::ASCII)
    $env:Path = "$CollisionDir;$SavedProcessPath"
    $userPathBeforeCollision = [Environment]::GetEnvironmentVariable('Path', 'User')
    $collision = Invoke-PowerShellFile $Bootstrap @('-BinDir', $BinDir)
    if ($collision.Code -eq 2) { Ok 'unrelated wsp collision blocks install' } else { Fail-Test 'unrelated wsp collision blocks install' $collision.Output }
    Contains $collision.Output 'UNRELATED_WSP_COMMAND_EXISTS:' 'collision diagnostic explicit'
    if (-not (Test-Path -LiteralPath $Target)) { Ok 'collision creates no Product launcher' } else { Fail-Test 'collision creates no Product launcher' }
    if ([string]::Equals($userPathBeforeCollision, [Environment]::GetEnvironmentVariable('Path', 'User'), [System.StringComparison]::Ordinal)) {
        Ok 'collision preserves User PATH'
    } else {
        Fail-Test 'collision preserves User PATH'
    }

    $env:Path = $SavedProcessPath
    Remove-Item -LiteralPath $collisionLauncher -Force

    $first = Invoke-PowerShellFile $Bootstrap @('-BinDir', $BinDir)
    if ($first.Code -eq 0) { Ok 'first Windows install succeeds' } else { Fail-Test 'first Windows install succeeds' $first.Output }
    Contains $first.Output 'RESULT: REGISTERED' 'Windows launcher first registration'
    Contains $first.Output 'CMD_SURFACE: PASS' 'bootstrap verifies CMD surface'
    Contains $first.Output 'POWERSHELL_SURFACE: PASS' 'bootstrap verifies PowerShell surface'
    Contains $first.Output 'PLATFORM_SUPPORT: IMPLEMENTED_CI_VERIFIED' 'Windows support claim stays CI-scoped'
    Contains $first.Output 'PHYSICAL_WINDOWS_ACCEPTANCE: PENDING' 'physical Windows acceptance not self-promoted'

    if (Test-Path -LiteralPath $Target -PathType Leaf) { Ok 'wsp.cmd launcher created' } else { Fail-Test 'wsp.cmd launcher created' }
    if ((Get-Content -LiteralPath $Target -TotalCount 2)[1] -eq 'REM WSP_PRODUCT_LAUNCHER_V1') { Ok 'launcher Product marker present' } else { Fail-Test 'launcher Product marker present' }

    $userPathAfterFirst = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (Test-PathEntry $userPathAfterFirst $BinDir) { Ok 'Windows User PATH appended' } else { Fail-Test 'Windows User PATH appended' $userPathAfterFirst }
    if ((Count-PathEntry $userPathAfterFirst $BinDir) -eq 1) { Ok 'Windows User PATH entry unique' } else { Fail-Test 'Windows User PATH entry unique' $userPathAfterFirst }
    if (-not [string]::IsNullOrWhiteSpace($SavedUserPath) -and $userPathAfterFirst.StartsWith($SavedUserPath, [System.StringComparison]::Ordinal)) {
        Ok 'existing User PATH content preserved'
    } elseif ([string]::IsNullOrWhiteSpace($SavedUserPath)) {
        Ok 'existing User PATH content preserved'
    } else {
        Fail-Test 'existing User PATH content preserved' $userPathAfterFirst
    }

    $launcherBefore = [System.IO.File]::ReadAllText($Target)
    $env:Path = "$BinDir;$SavedProcessPath"

    $resolved = Get-Command wsp -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($resolved -and [string]::Equals([System.IO.Path]::GetFullPath($resolved.Source), [System.IO.Path]::GetFullPath($Target), [System.StringComparison]::OrdinalIgnoreCase)) {
        Ok 'PowerShell Get-Command resolves Product wsp'
    } else {
        Fail-Test 'PowerShell Get-Command resolves Product wsp' (($resolved | Out-String).Trim())
    }

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $cmdOutput = & cmd.exe /d /s /c "where wsp && wsp version" 2>&1
        $cmdCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($cmdCode -eq 0) { Ok 'CMD directly invokes wsp version' } else { Fail-Test 'CMD directly invokes wsp version' (($cmdOutput | Out-String).Trim()) }
    Contains (($cmdOutput | Out-String)) 'PLATFORM_FAMILY: Windows' 'CMD sees Windows host classification'

    $psExe = Get-Command pwsh.exe -ErrorAction SilentlyContinue
    if (-not $psExe) { $psExe = Get-Command powershell.exe -ErrorAction SilentlyContinue }
    $psCommand = '$c = Get-Command wsp -CommandType Application -ErrorAction SilentlyContinue; if (-not $c) { exit 41 }; & wsp version; exit $LASTEXITCODE'
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $psOutput = & $psExe.Source -NoProfile -ExecutionPolicy Bypass -Command $psCommand 2>&1
        $psCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($psCode -eq 0) { Ok 'PowerShell directly invokes wsp version' } else { Fail-Test 'PowerShell directly invokes wsp version' (($psOutput | Out-String).Trim()) }
    Contains (($psOutput | Out-String)) 'TERMINAL_SURFACES: CMD,PowerShell' 'same wsp command represents both Windows terminal surfaces'

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $argOutput = & wsp 'argument with spaces' 2>&1
        $argCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($argCode -eq 2) { Ok 'Windows launcher forwards exit code' } else { Fail-Test 'Windows launcher forwards exit code' (($argOutput | Out-String).Trim()) }
    Contains (($argOutput | Out-String)) 'unknown command: argument with spaces' 'Windows launcher forwards spaced argument intact'

    $env:WSP_STATE_HOME = $StateHome
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $initOutput = & wsp init $Workspace 2>&1
        $initCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($initCode -eq 0) { Ok 'Windows adapter forwards init with spaced workspace path' } else { Fail-Test 'Windows adapter forwards init with spaced workspace path' (($initOutput | Out-String).Trim()) }
    Contains (($initOutput | Out-String)) 'RESULT: CREATED' 'Windows native route reaches shared Bash config behavior'
    if (Test-Path -LiteralPath (Join-Path $Workspace '.wsp.properties') -PathType Leaf) { Ok 'Windows route creates workspace-local config' } else { Fail-Test 'Windows route creates workspace-local config' }

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $validateOutput = & wsp config validate 2>&1
        $validateCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($validateCode -eq 0) { Ok 'Windows route validates shared config lifecycle' } else { Fail-Test 'Windows route validates shared config lifecycle' (($validateOutput | Out-String).Trim()) }
    Contains (($validateOutput | Out-String)) 'RESULT: VALID' 'Windows shared config remains valid'

    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $doctorOutput = & wsp doctor 2>&1
        $doctorCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($doctorCode -eq 0) { Ok 'Windows wsp doctor passes configured installed state' } else { Fail-Test 'Windows wsp doctor passes configured installed state' (($doctorOutput | Out-String).Trim()) }
    $doctorText = ($doctorOutput | Out-String)
    Contains $doctorText 'PLATFORM_HOST: Windows-native' 'doctor reports Windows native host'
    Contains $doctorText 'PLATFORM_ENVIRONMENT: native-terminal-adapter' 'doctor reports native terminal adapter'
    Contains $doctorText 'COMMAND_REGISTRATION: PASS' 'doctor verifies Windows launcher'
    Contains $doctorText 'PATH_REGISTRATION: PASS' 'doctor verifies persistent Windows User PATH'
    Contains $doctorText 'PHYSICAL_WINDOWS_ACCEPTANCE: PENDING' 'doctor preserves physical acceptance pending state'
    Contains $doctorText 'OVERALL: PASS' 'doctor overall pass after Windows install/init'

    $second = Invoke-PowerShellFile $Bootstrap @('-BinDir', $BinDir)
    if ($second.Code -eq 0) { Ok 'second Windows install succeeds' } else { Fail-Test 'second Windows install succeeds' $second.Output }
    Contains $second.Output 'RESULT: NO_CHANGE_REQUIRED' 'Windows install idempotent'
    if ([System.IO.File]::ReadAllText($Target) -eq $launcherBefore) { Ok 'idempotent install preserves launcher bytes' } else { Fail-Test 'idempotent install preserves launcher bytes' }
    $userPathAfterSecond = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($userPathAfterSecond -eq $userPathAfterFirst) { Ok 'idempotent install preserves User PATH' } else { Fail-Test 'idempotent install preserves User PATH' $userPathAfterSecond }
    if ((Count-PathEntry $userPathAfterSecond $BinDir) -eq 1) { Ok 'idempotent install does not duplicate PATH entry' } else { Fail-Test 'idempotent install does not duplicate PATH entry' $userPathAfterSecond }

    $adapterText = Get-Content -LiteralPath (Join-Path $Product 'scripts\wsp-windows.ps1') -Raw
    $bootstrapText = Get-Content -LiteralPath $Bootstrap -Raw
    if ($adapterText -notmatch 'CODEX_THREAD_ID|WORKSPACE_SESSION_ID|Claude' -and $bootstrapText -notmatch 'CODEX_THREAD_ID|WORKSPACE_SESSION_ID|Claude') {
        Ok 'no provider identity logic added to Windows routing'
    } else {
        Fail-Test 'no provider identity logic added to Windows routing'
    }
} finally {
    [Environment]::SetEnvironmentVariable('Path', $SavedUserPath, 'User')
    $env:Path = $SavedProcessPath
    $env:WSP_STATE_HOME = $SavedStateHome
    Remove-Item -LiteralPath $Temp -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Output "RESULT: PASS=$Pass FAIL=$Fail"
if ($Fail -eq 0) {
    Write-Output 'STATUS: PASS'
    exit 0
}
Write-Output 'STATUS: FAIL'
exit 1
