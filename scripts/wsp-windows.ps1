param(
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$Arguments
)

$ErrorActionPreference = 'Stop'

function Fail-Wsp {
    param([string]$Reason)
    [Console]::Error.WriteLine('STATUS: BLOCKED')
    [Console]::Error.WriteLine("REASON: $Reason")
    exit 2
}

function Get-GitForWindowsTools {
    $gitCommand = Get-Command git.exe -ErrorAction SilentlyContinue
    if (-not $gitCommand) { $gitCommand = Get-Command git -ErrorAction SilentlyContinue }
    if (-not $gitCommand -or [string]::IsNullOrWhiteSpace($gitCommand.Source)) {
        Fail-Wsp 'GIT_FOR_WINDOWS_NOT_FOUND'
    }

    $gitPath = [System.IO.Path]::GetFullPath($gitCommand.Source)
    $gitDir = Split-Path -Parent $gitPath
    $gitRoot = Split-Path -Parent $gitDir

    $bashCandidates = @(
        (Join-Path $gitRoot 'bin\bash.exe'),
        (Join-Path $gitRoot 'usr\bin\bash.exe'),
        (Join-Path $gitDir 'bash.exe')
    )
    $cygpathCandidates = @(
        (Join-Path $gitRoot 'usr\bin\cygpath.exe'),
        (Join-Path $gitRoot 'bin\cygpath.exe')
    )

    $bash = $bashCandidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1
    $cygpath = $cygpathCandidates | Where-Object { Test-Path -LiteralPath $_ -PathType Leaf } | Select-Object -First 1

    if (-not $bash) { Fail-Wsp 'GIT_FOR_WINDOWS_BASH_NOT_FOUND' }
    if (-not $cygpath) { Fail-Wsp 'GIT_FOR_WINDOWS_CYGPATH_NOT_FOUND' }

    return [pscustomobject]@{
        Git = $gitPath
        Bash = [System.IO.Path]::GetFullPath($bash)
        Cygpath = [System.IO.Path]::GetFullPath($cygpath)
    }
}

function Convert-ToPosixPath {
    param([string]$PathValue, [string]$Cygpath)
    if ([string]::IsNullOrWhiteSpace($PathValue)) { return $PathValue }
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $converted = & $Cygpath '-u' $PathValue 2>$null
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    if ($code -ne 0 -or [string]::IsNullOrWhiteSpace(($converted | Out-String).Trim())) {
        Fail-Wsp "WINDOWS_PATH_CONVERSION_FAILED:$PathValue"
    }
    return (($converted | Out-String).Trim())
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

function Get-WindowsCommandHealth {
    $command = Get-Command wsp -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if (-not $command -or [string]::IsNullOrWhiteSpace($command.Source)) {
        return [pscustomobject]@{ Registration = 'ABSENT'; Path = ''; PersistentPath = 'MISSING' }
    }

    $source = [System.IO.Path]::GetFullPath($command.Source)
    $registration = if (Test-ProductLauncher $source) { 'PASS' } else { 'CONFLICT' }
    $dir = Split-Path -Parent $source
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    $pathState = if (Test-PathEntry $userPath $dir) { 'PASS' } else { 'MISSING' }
    return [pscustomobject]@{ Registration = $registration; Path = $source; PersistentPath = $pathState }
}

if ($env:OS -ne 'Windows_NT') {
    Fail-Wsp 'WINDOWS_NATIVE_ADAPTER_REQUIRES_WINDOWS'
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$productRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptDir '..'))
$corePath = [System.IO.Path]::GetFullPath((Join-Path $productRoot 'bin\wsp'))
$bootstrapPath = [System.IO.Path]::GetFullPath((Join-Path $scriptDir 'bootstrap-windows.ps1'))
if (-not (Test-Path -LiteralPath $corePath -PathType Leaf)) { Fail-Wsp 'BASH_CORE_NOT_FOUND' }

$commandName = if ($Arguments.Count -gt 0) { $Arguments[0].ToLowerInvariant() } else { 'help' }

if ($commandName -eq 'bootstrap') {
    if (-not (Test-Path -LiteralPath $bootstrapPath -PathType Leaf)) { Fail-Wsp 'WINDOWS_BOOTSTRAP_NOT_FOUND' }
    $shellCommand = Get-Command pwsh.exe -ErrorAction SilentlyContinue
    if (-not $shellCommand) { $shellCommand = Get-Command powershell.exe -ErrorAction SilentlyContinue }
    if (-not $shellCommand) { Fail-Wsp 'POWERSHELL_RUNTIME_NOT_FOUND' }

    $bootstrapArgs = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $bootstrapPath)
    if ($Arguments.Count -eq 1) {
        # default Windows user command directory
    } elseif ($Arguments.Count -eq 3 -and $Arguments[1] -eq '--bin-dir') {
        $bootstrapArgs += @('-BinDir', $Arguments[2])
    } else {
        Fail-Wsp 'USAGE:bootstrap [--bin-dir DIR]'
    }

    & $shellCommand.Source @bootstrapArgs
    exit $LASTEXITCODE
}

$tools = Get-GitForWindowsTools
$corePosix = Convert-ToPosixPath $corePath $tools.Cygpath
$productRootPosix = Convert-ToPosixPath $productRoot $tools.Cygpath

$coreArgs = @($Arguments)
if ($coreArgs.Count -gt 1 -and $coreArgs[0] -eq 'init') {
    $coreArgs[1] = Convert-ToPosixPath $coreArgs[1] $tools.Cygpath
} elseif ($coreArgs.Count -gt 2 -and $coreArgs[0] -eq 'repo' -and $coreArgs[1] -eq 'inspect') {
    $coreArgs[2] = Convert-ToPosixPath $coreArgs[2] $tools.Cygpath
}

$oldRoot = $env:WSP_ROOT
$oldHome = $env:HOME
$oldStateHome = $env:WSP_STATE_HOME
$env:WSP_ROOT = $productRootPosix
if (-not [string]::IsNullOrWhiteSpace($HOME)) {
    $env:HOME = Convert-ToPosixPath $HOME $tools.Cygpath
}
if (-not [string]::IsNullOrWhiteSpace($env:WSP_STATE_HOME)) {
    $env:WSP_STATE_HOME = Convert-ToPosixPath $env:WSP_STATE_HOME $tools.Cygpath
}
$env:WSP_WINDOWS_NATIVE_ADAPTER = '1'

$bashShim = @'
uname() {
  case "${1:-}" in
    -s) printf 'Linux\n' ;;
    -m) /usr/bin/uname -m ;;
    *) /usr/bin/uname "$@" ;;
  esac
}
source "$1" "${@:2}"
'@

function Invoke-CoreCaptured {
    param([string[]]$ForwardArgs)
    $oldPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $output = & $tools.Bash '--noprofile' '--norc' '-c' $bashShim 'wsp-windows-native' $corePosix @ForwardArgs 2>&1
        $code = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $oldPreference
    }
    return [pscustomobject]@{ Output = @($output | ForEach-Object { $_.ToString() }); Code = $code }
}

try {
    if ($commandName -eq 'version') {
        $result = Invoke-CoreCaptured $coreArgs
        foreach ($line in $result.Output) {
            if ($line -match '^PLATFORM_FAMILY:' -or $line -match '^PLATFORM_ENVIRONMENT:') { continue }
            Write-Output $line
        }
        Write-Output 'PLATFORM_HOST: Windows-native'
        Write-Output 'PLATFORM_FAMILY: Windows'
        Write-Output 'PLATFORM_ENVIRONMENT: native-terminal-adapter'
        Write-Output 'TERMINAL_SURFACES: CMD,PowerShell'
        Write-Output 'PLATFORM_SUPPORT: IMPLEMENTED_CI_VERIFIED'
        Write-Output 'RUNTIME_ADAPTER: Git-for-Windows-Bash'
        Write-Output 'PHYSICAL_WINDOWS_ACCEPTANCE: PENDING'
        exit $result.Code
    }

    if ($commandName -eq 'doctor') {
        $result = Invoke-CoreCaptured $coreArgs
        $otherBlocked = $false
        foreach ($line in $result.Output) {
            if ($line -match '^PLATFORM_' -or $line -match '^COMMAND_REGISTRATION:' -or $line -match '^COMMAND_PATH:' -or $line -match '^PATH_REGISTRATION:' -or $line -match '^OVERALL:') { continue }
            Write-Output $line
            if ($line -match '^DEPENDENCY_.+: MISSING$' -or
                $line -eq 'PRODUCT_LAYOUT: BLOCKED' -or
                $line -match '^WORKSPACE_CONFIGURATION: (ABSENT|INVALID_BINDING|BROKEN_BINDING)$' -or
                ($line -match '^CONFIG_STATE: (.+)$' -and $Matches[1] -ne 'VALID')) {
                $otherBlocked = $true
            }
        }

        $health = Get-WindowsCommandHealth
        Write-Output 'PLATFORM_HOST: Windows-native'
        Write-Output 'PLATFORM_FAMILY: Windows'
        Write-Output 'PLATFORM_ENVIRONMENT: native-terminal-adapter'
        Write-Output 'TERMINAL_SURFACES: CMD,PowerShell'
        Write-Output 'PLATFORM_SUPPORT: IMPLEMENTED_CI_VERIFIED'
        Write-Output 'RUNTIME_ADAPTER: Git-for-Windows-Bash'
        Write-Output 'PHYSICAL_WINDOWS_ACCEPTANCE: PENDING'
        Write-Output "COMMAND_REGISTRATION: $($health.Registration)"
        if (-not [string]::IsNullOrWhiteSpace($health.Path)) { Write-Output "COMMAND_PATH: $($health.Path)" }
        Write-Output "PATH_REGISTRATION: $($health.PersistentPath)"
        Write-Output 'NETWORK_MUTATION: NONE'
        Write-Output 'MANAGED_GIT_MUTATION: NONE'

        if ($otherBlocked -or $health.Registration -ne 'PASS' -or $health.PersistentPath -ne 'PASS') {
            Write-Output 'OVERALL: BLOCKED'
            exit 2
        }
        Write-Output 'OVERALL: PASS'
        exit 0
    }

    & $tools.Bash '--noprofile' '--norc' '-c' $bashShim 'wsp-windows-native' $corePosix @coreArgs
    exit $LASTEXITCODE
} finally {
    $env:WSP_ROOT = $oldRoot
    $env:HOME = $oldHome
    $env:WSP_STATE_HOME = $oldStateHome
    Remove-Item Env:WSP_WINDOWS_NATIVE_ADAPTER -ErrorAction SilentlyContinue
}
