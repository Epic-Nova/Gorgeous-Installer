package updater

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unicode/utf16"
)

type binaryUpdateHandoff struct {
	ParentPID                                                int
	Source, Destination, Fallback, ErrorPath, StageDirectory string
}

// Paths are JSON data rather than interpolated PowerShell or cmd.exe syntax.
// Wait for the actual owner to exit: a fixed sleep cannot guarantee Windows has released its executable.
func windowsBinaryUpdateCommand(handoff binaryUpdateHandoff) (*exec.Cmd, error) {
	for _, path := range []string{handoff.Source, handoff.Destination, handoff.ErrorPath, handoff.StageDirectory} {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("update handoff requires absolute paths: %q", path)
		}
	}
	data, err := json.Marshal(handoff)
	if err != nil {
		return nil, err
	}
	script := `$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$config = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + base64.StdEncoding.EncodeToString(data) + `')) | ConvertFrom-Json
$backup = Join-Path $config.StageDirectory 'previous-installer.exe'
$relaunch = $config.Destination
$failed = $false
try {
    if ($config.ParentPID -gt 0) {
        $owner = Get-Process -Id $config.ParentPID -ErrorAction SilentlyContinue
        if ($null -ne $owner -and -not $owner.WaitForExit(60000)) { throw 'Installer did not exit within 60 seconds; update was not applied.' }
    }
    if (Test-Path -LiteralPath $config.Destination) {
        Copy-Item -LiteralPath $config.Destination -Destination $backup -Force
    }
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while ($true) {
        try { Copy-Item -LiteralPath $config.Source -Destination $config.Destination -Force; break }
        catch { if ([DateTime]::UtcNow -ge $deadline) { throw }; Start-Sleep -Milliseconds 250 }
    }
    if (Test-Path -LiteralPath $config.ErrorPath) { Remove-Item -LiteralPath $config.ErrorPath -Force }
    $updated = Start-Process -FilePath $config.Destination -WorkingDirectory ([IO.Path]::GetDirectoryName($config.Destination)) -PassThru -ErrorAction Stop
    if ($updated.WaitForExit(1000) -and $updated.ExitCode -ne 0) { throw ('Updated installer exited during startup: ' + $updated.ExitCode) }
} catch {
    $failed = $true
    $message = $_.Exception.ToString()
    if (Test-Path -LiteralPath $backup) {
        try { Copy-Item -LiteralPath $backup -Destination $config.Destination -Force } catch { $message += "` + "`n" + `Rollback: " + $_.Exception.Message }
    }
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($config.ErrorPath)) | Out-Null
    [IO.File]::WriteAllText($config.ErrorPath, $message)
    $relaunch = $config.Fallback
    try {
        if (Test-Path -LiteralPath $relaunch) { Start-Process -FilePath $relaunch -WorkingDirectory ([IO.Path]::GetDirectoryName($relaunch)) -ErrorAction Stop }
    } catch { [IO.File]::AppendAllText($config.ErrorPath, "` + "`n" + `Previous installer could not restart: " + $_.Exception.Message) }
} finally {
    $stage = [IO.Path]::GetFullPath($config.StageDirectory).TrimEnd([char[]]'\/')
    $tempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd([char[]]'\/')
    if ([IO.Path]::GetDirectoryName($stage) -eq $tempRoot -and [IO.Path]::GetFileName($stage).StartsWith('gorgeous-installer-extracted-')) {
        Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue
    }
}
if ($failed) { exit 1 }
`
	encoded := utf16.Encode([]rune(script))
	bytes := make([]byte, len(encoded)*2)
	for i, unit := range encoded {
		binary.LittleEndian.PutUint16(bytes[i*2:], unit)
	}
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(powershell); err != nil {
		powershell = "powershell.exe"
	}
	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", base64.StdEncoding.EncodeToString(bytes))
	configureUpdateHelper(cmd)
	return cmd, nil
}
