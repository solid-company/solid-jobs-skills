<#
.SYNOPSIS
  Download and install the sjctl binary from GitHub Releases (Windows).

.DESCRIPTION
  Used by the jobs-* skills when sjctl.exe is not already on PATH. Safe to
  re-run: it skips the download if the installed version already matches.

  Environment overrides:
    SJCTL_REPO        owner/repo to pull releases from (default below)
    SJCTL_VERSION     release tag to install, e.g. v0.1.0 (default: latest)
    SJCTL_BIN_DIR     install directory (default: ~/.solid-jobs-skills\bin)
    SJCTL_SKIP_COSIGN set to 1 to skip cosign signature verification
#>
$ErrorActionPreference = "Stop"

$repo    = if ($env:SJCTL_REPO) { $env:SJCTL_REPO } else { "solid-company/solid-jobs-skills" }
$binDir  = if ($env:SJCTL_BIN_DIR) { $env:SJCTL_BIN_DIR } else { Join-Path $HOME ".solid-jobs-skills\bin" }
$target  = if ($env:SJCTL_VERSION) { $env:SJCTL_VERSION } else { "latest" }
$binPath = Join-Path $binDir "sjctl.exe"

# --- detect arch -------------------------------------------------------------
$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
  "AMD64" { "amd64" }
  # No windows/arm64 release is built; Windows on ARM runs the amd64 binary
  # under x64 emulation, so map to amd64 rather than 404 on a missing asset.
  "ARM64" { "amd64" }
  default { throw "install-sjctl: unsupported arch $($env:PROCESSOR_ARCHITECTURE)" }
}

# --- resolve target tag ------------------------------------------------------
$api = "https://api.github.com/repos/$repo/releases"
if ($target -eq "latest") {
  $tag = (Invoke-RestMethod -Uri "$api/latest" -Headers @{ "User-Agent" = "sjctl-installer" }).tag_name
  if (-not $tag) { throw "install-sjctl: could not resolve latest release for $repo" }
} else {
  $tag = $target
}

# --- skip if already at target -----------------------------------------------
if (Test-Path $binPath) {
  $current = (& $binPath version 2>$null)
  # `sjctl version` prints main.version, stamped from GoReleaser's {{ .Version }}
  # which strips the leading v (0.1.0), so compare against the tag without it.
  if ($current -eq ($tag -replace '^v','')) { Write-Output $binPath; exit 0 }
}

# --- download, verify, extract ----------------------------------------------
$asset = "sjctl_windows_${arch}.zip"
$base  = "https://github.com/$repo/releases/download/$tag"
$tmp   = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ([guid]::NewGuid()))
try {
  $zip = Join-Path $tmp $asset
  Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip -UseBasicParsing
  $sumFile = Join-Path $tmp "checksums.txt"
  Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sumFile -UseBasicParsing

  # --- verify the cosign signature over checksums.txt ------------------------
  # The sha256 match only proves the asset agrees with checksums.txt fetched
  # over the same channel. The keyless cosign signature proves checksums.txt was
  # produced by this repo's release workflow. Verified when cosign is on PATH;
  # set SJCTL_SKIP_COSIGN=1 to bypass (not recommended).
  if ($env:SJCTL_SKIP_COSIGN -eq "1") {
    Write-Warning "install-sjctl: skipping cosign verification (SJCTL_SKIP_COSIGN=1)"
  } elseif (Get-Command cosign -ErrorAction SilentlyContinue) {
    $sig = Join-Path $tmp "checksums.txt.sig"
    $cert = Join-Path $tmp "checksums.txt.pem"
    Invoke-WebRequest -Uri "$base/checksums.txt.sig" -OutFile $sig -UseBasicParsing
    Invoke-WebRequest -Uri "$base/checksums.txt.pem" -OutFile $cert -UseBasicParsing
    & cosign verify-blob `
      --certificate $cert `
      --signature $sig `
      --certificate-identity-regexp "^https://github.com/$repo/" `
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" `
      $sumFile 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "install-sjctl: cosign signature verification failed for checksums.txt" }
  } else {
    Write-Warning "install-sjctl: cosign not found; skipping signature verification (install cosign or set SJCTL_SKIP_COSIGN=1 to silence)"
  }

  $expected = (Select-String -Path $sumFile -Pattern ([regex]::Escape($asset)) |
    Select-Object -First 1).Line.Split(" ")[0]
  $actual = (Get-FileHash -Algorithm SHA256 -Path $zip).Hash.ToLower()
  if ($expected -ne $actual) { throw "install-sjctl: checksum verification failed for $asset" }

  Expand-Archive -Path $zip -DestinationPath $tmp -Force
  New-Item -ItemType Directory -Force -Path $binDir | Out-Null
  Copy-Item -Path (Join-Path $tmp "sjctl.exe") -Destination $binPath -Force

  Write-Output $binPath
} finally {
  Remove-Item -Recurse -Force $tmp
}
