# Build wmr for this machine, or for every supported platform with:
#   .\build.ps1 all
[CmdletBinding()]
param([ValidateSet('local', 'all')][string]$Target = 'local')

$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

function Get-Version {
    if ($env:VERSION) { return $env:VERSION }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) { return 'dev' }
    # git writes to stderr when this is not a repository; keep that from
    # tripping $ErrorActionPreference = 'Stop'.
    $prev = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try { $described = & git describe --tags --always --dirty 2>&1 } finally { $ErrorActionPreference = $prev }
    if ($LASTEXITCODE -ne 0) { return 'dev' }
    $text = ($described | Select-Object -First 1 | Out-String).Trim()
    if ($text) { return $text } else { return 'dev' }
}

$version = Get-Version
$ldflags = "-s -w -X main.version=$version"
$pkg = './cmd/wmr'
$out = 'dist'

function Build-Target([string]$os, [string]$arch) {
    $ext = ''
    if ($os -eq 'windows') { $ext = '.exe' }
    $name = "wmr-$os-$arch$ext"
    Write-Host "  $name"
    $env:GOOS = $os; $env:GOARCH = $arch; $env:CGO_ENABLED = '0'
    go build -trimpath -ldflags $ldflags -o (Join-Path $out $name) $pkg
    if ($LASTEXITCODE -ne 0) { throw "build failed for $os/$arch" }
}

# Leftover tool output breaks the build in a way that does not explain itself:
# a stray "x.cleaned.go" beside "x.go" is a duplicate of every declaration in
# it, so the compiler reports a redeclaration and never mentions the real
# cause. Name it here instead. These are not deleted automatically -- they are
# the user's files, even if the tool wrote them.
$strays = Get-ChildItem -Recurse -File -Filter '*.cleaned.*' |
    Where-Object { $_.FullName -notmatch '\\\.git\\' }
if ($strays) {
    Write-Host 'Leftover output from a previous "wmr clean" run:'
    $strays | ForEach-Object { Write-Host ("  " + (Resolve-Path -Relative $_.FullName)) }
    Write-Host 'These are copies of their neighbours and will break the build. Remove them and re-run.'
    throw 'stray .cleaned.* files present'
}

# gofmt reports unformatted files on stdout and says nothing when clean, so an
# empty result is the pass condition. CI gates on this too; catching it here
# saves a round trip.
$unformatted = (gofmt -l . | Where-Object { $_ })
if ($unformatted) {
    Write-Host 'These files need gofmt:'
    $unformatted | ForEach-Object { Write-Host "  $_" }
    throw 'gofmt check failed'
}

go vet ./...
if ($LASTEXITCODE -ne 0) { throw 'go vet failed' }

go test ./...
if ($LASTEXITCODE -ne 0) { throw 'tests failed' }

$savedOS = $env:GOOS; $savedArch = $env:GOARCH
try {
    if ($Target -eq 'all') {
        if (Test-Path $out) { Remove-Item -Recurse -Force $out }
        New-Item -ItemType Directory -Path $out | Out-Null
        Write-Host "Building $version for all platforms:"
        foreach ($t in 'windows/amd64', 'windows/arm64', 'linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64') {
            $parts = $t.Split('/')
            Build-Target $parts[0] $parts[1]
        }
        # Hash into a variable first: piping straight to Out-File would try to
        # hash the sums file while it is open for writing.
        $sums = Get-ChildItem $out -File | Where-Object { $_.Name -ne 'SHA256SUMS' } |
            ForEach-Object { "{0}  {1}" -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower(), $_.Name }
        # Written with .NET rather than Set-Content: on Windows PowerShell 5.1
        # "-Encoding utf8" emits a BOM and CRLF line endings, either of which
        # makes `sha256sum -c SHA256SUMS` fail on Linux and macOS.
        $lf = ($sums -join "`n") + "`n"
        [System.IO.File]::WriteAllText(
            (Join-Path $PSScriptRoot (Join-Path $out 'SHA256SUMS')),
            $lf,
            (New-Object System.Text.UTF8Encoding($false)))
        Write-Host "Done -> $out\"
    }
    else {
        $env:CGO_ENABLED = '0'
        $exe = 'wmr'
        if ((go env GOOS) -eq 'windows') { $exe = 'wmr.exe' }
        go build -trimpath -ldflags $ldflags -o $exe $pkg
        if ($LASTEXITCODE -ne 0) { throw 'build failed' }
        Write-Host "Built .\$exe ($version)"
    }
}
finally {
    $env:GOOS = $savedOS; $env:GOARCH = $savedArch; $env:CGO_ENABLED = $null
}
