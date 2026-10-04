//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func psTLStudioQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func launchTLStudioUpdater(candidate tlStudioUpdateCandidate, project string) error {
	executable, err := os.Executable()
	if err != nil { return fmt.Errorf("locate TL Studio executable: %w", err) }
	if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil { executable = resolved }
	installDir := filepath.Dir(executable)

	probe, err := os.CreateTemp(installDir, ".tl-studio-update-write-*")
	if err != nil { return fmt.Errorf("TL Studio install directory is not writable: %w", err) }
	probe.Close()
	_ = os.Remove(probe.Name())

	scriptFile, err := os.CreateTemp("", "tl-studio-update-*.ps1")
	if err != nil { return fmt.Errorf("create updater script: %w", err) }
	scriptPath := scriptFile.Name()
	scriptFile.Close()

	projectLaunch := "$launchArgs = @()"
	if strings.TrimSpace(project) != "" {
		projectLaunch = fmt.Sprintf("$launchArgs = @('--project', %s)", psTLStudioQuote(project))
	}

	script := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$host.UI.RawUI.WindowTitle = 'TL Studio Updater'
$assetUrl = %s
$sumsUrl = %s
$assetName = %s
$targetVersion = %s
$installDir = %s
$exePath = %s
$oldPid = %d
$projectDir = %s
%s
$tempRoot = Join-Path $env:TEMP ('tl-studio-update-' + [Guid]::NewGuid().ToString('N'))
$archive = Join-Path $tempRoot $assetName
$sums = Join-Path $tempRoot 'SHA256SUMS.txt'
$stage = Join-Path $tempRoot 'stage'
try {
  New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null
  Write-Host ('Downloading TL Studio ' + $targetVersion + '...') -ForegroundColor Cyan
  Invoke-WebRequest -UseBasicParsing -Uri $assetUrl -OutFile $archive
  Invoke-WebRequest -UseBasicParsing -Uri $sumsUrl -OutFile $sums

  $line = Get-Content -LiteralPath $sums | Where-Object { $_.Trim().EndsWith($assetName) } | Select-Object -First 1
  if (-not $line) { throw ('SHA256SUMS.txt does not contain ' + $assetName) }
  $expected = (($line.Trim() -split '\s+')[0]).ToLowerInvariant()
  $actual = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actual -ne $expected) { throw ('SHA-256 mismatch. Expected ' + $expected + ', got ' + $actual) }
  Write-Host 'SHA-256 verified.' -ForegroundColor Green

  New-Item -ItemType Directory -Path $stage -Force | Out-Null
  Expand-Archive -LiteralPath $archive -DestinationPath $stage -Force
  $stagedExe = Join-Path $stage 'tl-studio.exe'
  if (-not (Test-Path -LiteralPath $stagedExe)) { throw 'Release archive does not contain tl-studio.exe' }

  $writeProbe = Join-Path $installDir ('.tl-studio-update-probe-' + [Guid]::NewGuid().ToString('N'))
  Set-Content -LiteralPath $writeProbe -Value 'ok' -NoNewline
  Remove-Item -LiteralPath $writeProbe -Force

  Write-Host 'Release verified. TL Studio will close and restart automatically.' -ForegroundColor Yellow
  Start-Sleep -Milliseconds 1400
  Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
  for ($i = 0; $i -lt 50; $i++) {
    if (-not (Get-Process -Id $oldPid -ErrorAction SilentlyContinue)) { break }
    Start-Sleep -Milliseconds 100
  }

  & robocopy.exe $stage $installDir /E /R:2 /W:1 /NFL /NDL /NJH /NJS /NP | Out-Null
  if ($LASTEXITCODE -ge 8) { throw ('robocopy failed with exit code ' + $LASTEXITCODE) }
  if (-not (Test-Path -LiteralPath $exePath)) { throw 'Updated tl-studio.exe was not found after copy' }

  if ($projectDir -and (Test-Path -LiteralPath $projectDir)) {
    Start-Process -FilePath $exePath -ArgumentList $launchArgs -WorkingDirectory $projectDir
  } else {
    Start-Process -FilePath $exePath -ArgumentList $launchArgs -WorkingDirectory $installDir
  }
  Write-Host ('TL Studio ' + $targetVersion + ' installed successfully.') -ForegroundColor Green
  Start-Sleep -Seconds 2
} catch {
  Write-Host ''
  Write-Host ('Update failed: ' + $_.Exception.Message) -ForegroundColor Red
  Read-Host 'Press Enter to close'
} finally {
  Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}
`,
		psTLStudioQuote(candidate.AssetURL),
		psTLStudioQuote(candidate.SumsURL),
		psTLStudioQuote(candidate.AssetName),
		psTLStudioQuote(candidate.LatestVersion),
		psTLStudioQuote(installDir),
		psTLStudioQuote(executable),
		os.Getpid(),
		psTLStudioQuote(project),
		projectLaunch,
	)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write updater script: %w", err)
	}

	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Dir = installDir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000010 | 0x00000200}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return fmt.Errorf("start PowerShell updater: %w", err)
	}
	_ = cmd.Process.Release()
	go func() {
		time.Sleep(10 * time.Minute)
		_ = os.Remove(scriptPath)
	}()
	return nil
}
