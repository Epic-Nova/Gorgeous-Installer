//go:build windows

package selfinstall

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const uninstallKey = "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\GorgeousInstaller"
const projectKey = "Software\\Classes\\GorgeousInstaller.ProjectFile"

type windowsPlatform struct{ programs, uninstall, project, settings string }

func currentPlatform() (platform, error) {
	programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return nil, fmt.Errorf("locate this user's Start menu: %w", err)
	}
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return windowsPlatform{programs, uninstallKey, projectKey, filepath.Join(configDirectory, "GorgeousThings", "Installer.json")}, nil
}
func samePath(a, b string) bool        { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func quotePS(value string) string      { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
func quoteCommand(value string) string { return "\"" + value + "\"" }
func powershell() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}
func hiddenCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(powershell(), args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}
func runPowerShell(script string) ([]byte, error) {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	output, err := hiddenCommand("-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(data)).CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("Windows shell integration: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}
func (p windowsPlatform) shortcut() string {
	return filepath.Join(p.programs, "Gorgeous Installer.lnk")
}
func (p windowsPlatform) publish(result Result, version string) (func() error, error) {
	return p.publishWithFault(result, version, nil)
}
func (p windowsPlatform) publishWithFault(result Result, version string, fault func(string) error) (func() error, error) {
	transaction, err := newShellTransaction(p.uninstall)
	if err != nil {
		return nil, err
	}
	return transaction.rollback, p.publishInto(result, version, transaction, fault)
}
func (p windowsPlatform) publishInto(result Result, version string, transaction *shellTransaction, fault func(string) error) error {
	checkpoint := func(stage string) error {
		if fault != nil {
			return fault(stage)
		}
		return nil
	}
	if err := os.MkdirAll(p.programs, 0755); err != nil {
		return err
	}
	scriptPath := filepath.Join(result.Directory, "uninstall-gorgeous.ps1")
	// Windows PowerShell 5.1 needs the BOM to preserve non-ASCII user paths.
	scriptBytes := append([]byte{0xef, 0xbb, 0xbf}, []byte(p.uninstallScript(result))...)
	if err := transaction.writeFile(scriptPath, scriptBytes, 0644); err != nil {
		return err
	}
	if err := checkpoint("script"); err != nil {
		return err
	}
	script := "$ErrorActionPreference='Stop'; $shell=New-Object -ComObject WScript.Shell; $link=$shell.CreateShortcut(" + quotePS(p.shortcut()) + "); $link.TargetPath=" + quotePS(result.Executable) + "; $link.Arguments='--gui'; $link.WorkingDirectory=" + quotePS(result.Directory) + "; $link.IconLocation=" + quotePS(result.Executable+",0") + "; $link.Description='Gorgeous Installer'; $link.Save()"
	if err := transaction.mutateFile(p.shortcut(), true, func() error { _, err := runPowerShell(script); return err }); err != nil {
		return err
	}
	if err := checkpoint("shortcut"); err != nil {
		return err
	}
	key, err := transaction.createKey(p.uninstall)
	if err != nil {
		return err
	}
	defer key.Close()
	uninstallCommand := quoteCommand(powershell()) + " -NoProfile -ExecutionPolicy Bypass -File " + quoteCommand(scriptPath)
	values := map[string]string{"DisplayName": "Gorgeous Installer", "DisplayVersion": version, "Publisher": "Epic Nova", "InstallLocation": result.Directory, "DisplayIcon": quoteCommand(result.Executable) + ",0", "UninstallString": uninstallCommand, "QuietUninstallString": uninstallCommand + " -Quiet"}
	for name, value := range values {
		if err := transaction.setString(key, p.uninstall, name, value); err != nil {
			return err
		}
	}
	for _, name := range []string{"NoModify", "NoRepair"} {
		if err := transaction.setDWord(key, p.uninstall, name, 1); err != nil {
			return err
		}
	}
	if err := checkpoint("uninstall"); err != nil {
		return err
	}
	command, err := transaction.createKey(p.project + "\\shell\\open\\command")
	if err != nil {
		return err
	}
	defer command.Close()
	if err := transaction.setString(command, p.project+"\\shell\\open\\command", "", quoteCommand(result.Executable)+" \"%1\""); err != nil {
		return err
	}
	friendly, err := transaction.createKey(p.project)
	if err != nil {
		return err
	}
	defer friendly.Close()
	if err := transaction.setString(friendly, p.project, "", "Gorgeous Installer"); err != nil {
		return err
	}
	openWithPath := p.project[:strings.LastIndex(p.project, "\\")] + "\\.uproject\\OpenWithProgids"
	openWith, err := transaction.createKey(openWithPath)
	if err != nil {
		return err
	}
	defer openWith.Close()
	if err := transaction.setString(openWith, openWithPath, "GorgeousInstaller.ProjectFile", ""); err != nil {
		return err
	}
	return checkpoint("complete")
}
func removeKey(path string) error {
	err := registry.DeleteKey(registry.CURRENT_USER, path)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
func (p windowsPlatform) remove(Result) error {
	if err := os.Remove(p.shortcut()); err != nil && !os.IsNotExist(err) {
		return err
	}
	openWithPath := p.project[:strings.LastIndex(p.project, "\\")] + "\\.uproject\\OpenWithProgids"
	if key, err := registry.OpenKey(registry.CURRENT_USER, openWithPath, registry.SET_VALUE); err == nil {
		_ = key.DeleteValue("GorgeousInstaller.ProjectFile")
		key.Close()
	}
	_ = removeKey(openWithPath)
	for _, path := range []string{p.uninstall, p.project + "\\shell\\open\\command", p.project + "\\shell\\open", p.project + "\\shell", p.project} {
		if err := removeKey(path); err != nil {
			return err
		}
	}
	return nil
}
func (p windowsPlatform) uninstallScript(result Result) string {
	// Exact owned files only; never recursively delete the selected folder.
	return "param([int]$WaitForProcessId=0,[switch]$Quiet)\r\n$ErrorActionPreference='Stop'\r\ntry {\r\n" +
		"if($WaitForProcessId -gt 0){Wait-Process -Id $WaitForProcessId -ErrorAction SilentlyContinue}\r\n" +
		"$exe=" + quotePS(result.Executable) + "\r\nif(Test-Path -LiteralPath $exe){Remove-Item -LiteralPath $exe -Force}\r\n" +
		"$shortcut=" + quotePS(p.shortcut()) + "\r\nif(Test-Path -LiteralPath $shortcut){$shell=New-Object -ComObject WScript.Shell; if($shell.CreateShortcut($shortcut).TargetPath -eq $exe){Remove-Item -LiteralPath $shortcut -Force}}\r\n" +
		"$registration=" + quotePS("HKCU:\\"+p.uninstall) + "\r\n$ownsRegistration=$false\r\nif(Test-Path -LiteralPath $registration){$ownsRegistration=[String]::Equals((Get-ItemPropertyValue -LiteralPath $registration -Name InstallLocation)," + quotePS(result.Directory) + ",[StringComparison]::OrdinalIgnoreCase)}\r\n" +
		"if($ownsRegistration){\r\n" +
		"$config=" + quotePS(p.settings) + "\r\n$preferences=$null\r\nif(Test-Path -LiteralPath $config){try{$preferences=Get-Content -LiteralPath $config -Raw | ConvertFrom-Json}catch{$preferences=$null}}\r\n" +
		"$association=" + quotePS("HKCU:\\"+p.project[:strings.LastIndex(p.project, "\\")]+"\\.uproject") + "\r\n" +
		"if((Test-Path -LiteralPath $association) -and (Get-Item -LiteralPath $association).GetValue('') -eq 'GorgeousInstaller.ProjectFile'){$previous='Unreal.ProjectFile';if($preferences -and $preferences.prevUprojectCommand){$previous=$preferences.prevUprojectCommand};Set-Item -LiteralPath $association -Value $previous}\r\n" +
		"if($preferences){$preferences|Add-Member -NotePropertyName uprojectAssociated -NotePropertyValue $false -Force;$preferences|Add-Member -NotePropertyName prevUprojectCommand -NotePropertyValue '' -Force;$preferences|Add-Member -NotePropertyName installedNatively -NotePropertyValue $false -Force;[IO.File]::WriteAllText($config,($preferences | ConvertTo-Json -Depth 20),[Text.UTF8Encoding]::new($false))}\r\n" +
		"$openWith=" + quotePS("HKCU:\\"+p.project[:strings.LastIndex(p.project, "\\")]+"\\.uproject\\OpenWithProgids") + "\r\nif(Test-Path -LiteralPath $openWith){Remove-ItemProperty -LiteralPath $openWith -Name 'GorgeousInstaller.ProjectFile' -ErrorAction SilentlyContinue;$key=Get-Item -LiteralPath $openWith;if($key.ValueCount -eq 0 -and $key.SubKeyCount -eq 0){Remove-Item -LiteralPath $openWith}}\r\n" +
		"foreach($key in @(" + quotePS("HKCU:\\"+p.uninstall) + "," + quotePS("HKCU:\\"+p.project) + ")){if(Test-Path -LiteralPath $key){Remove-Item -LiteralPath $key -Recurse -Force}}\r\n}\r\n" +
		"foreach($file in @(" + quotePS(filepath.Join(result.Directory, manifestName)) + "," + quotePS(filepath.Join(result.Directory, "uninstall-gorgeous.ps1")) + ")){if(Test-Path -LiteralPath $file){Remove-Item -LiteralPath $file -Force}}\r\n" +
		"$directory=" + quotePS(result.Directory) + "\r\nif((Test-Path -LiteralPath $directory) -and @(Get-ChildItem -LiteralPath $directory -Force).Count -eq 0){Remove-Item -LiteralPath $directory}\r\n" +
		"} catch {if(!$Quiet){Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('Close Gorgeous Installer and retry uninstalling. '+$_.Exception.Message,'Gorgeous Installer uninstall')|Out-Null}; Write-Error $_.Exception.Message; exit 1}\r\n"
}

// Uninstall returns true when the caller must exit so its helper can delete it.
func Uninstall(directory string) (bool, error) {
	result, err := readInstallation(directory)
	if err != nil {
		return false, err
	}
	current, _ := os.Executable()
	if samePath(current, result.Executable) {
		command := hiddenCommand("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(directory, "uninstall-gorgeous.ps1"), "-WaitForProcessId", strconv.Itoa(os.Getpid()))
		if err := command.Start(); err != nil {
			return false, err
		}
		_ = command.Process.Release()
		return true, nil
	}
	output, err := hiddenCommand("-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", filepath.Join(directory, "uninstall-gorgeous.ps1"), "-Quiet").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("uninstall: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return false, nil
}
