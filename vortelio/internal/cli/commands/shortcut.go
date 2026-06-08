package commands

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vortelio/vortelio/internal/config"
	"github.com/vortelio/vortelio/internal/version"
)

// vortelio.ico is shipped with the binary so the OS shortcut/app entry has a
// proper icon without depending on any external file.
//
//go:embed vortelio.ico
var appIcon []byte

// InstallAppCommand creates the desktop / Start-Menu / Spotlight entry that opens
// the Vortelio GUI, so the app shows up in the OS search after install.
type InstallAppCommand struct{}

func NewInstallAppCommand() *InstallAppCommand   { return &InstallAppCommand{} }
func (c *InstallAppCommand) Name() string        { return "install-app" }
func (c *InstallAppCommand) Run(args []string) error {
	path, err := createAppShortcut(true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌  Impossibile creare la scorciatoia: %v\n", err)
		return nil
	}
	fmt.Printf("✅  Vortelio aggiunto al menu del sistema: %s\n", path)
	switch runtime.GOOS {
	case "windows":
		fmt.Println("    Cerca \"Vortelio\" nella barra di ricerca di Windows.")
	case "darwin":
		fmt.Println("    Cercalo con Spotlight (⌘+Spazio) o in Applicazioni.")
	default:
		fmt.Println("    Compare nel menu applicazioni del desktop.")
	}
	return nil
}

// shortcutMarker records what the last-created shortcut points to, so the entry
// is recreated only when the binary path or version changes (cheap on every run).
type shortcutMarker struct {
	Target  string `json:"target"`
	Version string `json:"version"`
}

func shortcutMarkerPath() string {
	return filepath.Join(config.HomeDir(), "app_shortcut.json")
}

// EnsureAppShortcut creates the OS app entry on first run (or after an upgrade),
// pointing the GUI launcher at the current binary. Best-effort and silent: it
// never blocks or fails a normal command.
func EnsureAppShortcut() {
	defer func() { _ = recover() }()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if data, err := os.ReadFile(shortcutMarkerPath()); err == nil {
		var m shortcutMarker
		if json.Unmarshal(data, &m) == nil && m.Target == exe && m.Version == version.Version {
			return // up to date — nothing to do
		}
	}
	if _, err := createAppShortcut(false); err != nil {
		return
	}
	if data, err := json.Marshal(shortcutMarker{Target: exe, Version: version.Version}); err == nil {
		_ = os.MkdirAll(config.HomeDir(), 0o755)
		_ = os.WriteFile(shortcutMarkerPath(), data, 0o644)
	}
}

// iconPath writes the embedded icon to the Vortelio home and returns its path.
func iconPath() string {
	name := "vortelio.ico"
	if runtime.GOOS != "windows" {
		name = "vortelio.png" // best-effort; ico bytes still display on most DEs
	}
	p := filepath.Join(config.HomeDir(), name)
	if _, err := os.Stat(p); err != nil {
		_ = os.MkdirAll(config.HomeDir(), 0o755)
		_ = os.WriteFile(p, appIcon, 0o644)
	}
	return p
}

// createAppShortcut creates the platform app entry pointing at "<binary> gui".
// Returns the created path. force is informational (the entry is always written).
func createAppShortcut(force bool) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "windows":
		return createWindowsShortcut(exe)
	case "darwin":
		return createMacApp(exe)
	default:
		return createLinuxDesktop(exe)
	}
}

// createWindowsShortcut writes a .lnk in the per-user Start Menu so "Vortelio"
// is searchable from the Windows search bar.
func createWindowsShortcut(exe string) (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "", fmt.Errorf("APPDATA non impostato")
	}
	lnk := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Vortelio.lnk")
	ico := iconPath()
	ps := strings.Join([]string{
		"$ws = New-Object -ComObject WScript.Shell;",
		fmt.Sprintf("$s = $ws.CreateShortcut(%s);", psQuote(lnk)),
		fmt.Sprintf("$s.TargetPath = %s;", psQuote(exe)),
		"$s.Arguments = 'gui';",
		fmt.Sprintf("$s.IconLocation = %s;", psQuote(ico)),
		fmt.Sprintf("$s.WorkingDirectory = %s;", psQuote(filepath.Dir(exe))),
		"$s.Description = 'Vortelio — AI locale';",
		"$s.Save();",
	}, " ")
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return lnk, nil
}

// psQuote single-quotes a string for PowerShell (doubling embedded quotes).
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// createMacApp builds a minimal ~/Applications/Vortelio.app launcher so Vortelio
// shows up in Spotlight and Launchpad.
func createMacApp(exe string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	app := filepath.Join(home, "Applications", "Vortelio.app")
	macOS := filepath.Join(app, "Contents", "MacOS")
	res := filepath.Join(app, "Contents", "Resources")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		return "", err
	}
	_ = os.MkdirAll(res, 0o755)
	_ = os.WriteFile(filepath.Join(res, "vortelio.icns"), appIcon, 0o644)

	launcher := "#!/bin/sh\nexec " + shQuote(exe) + " gui\n"
	runPath := filepath.Join(macOS, "Vortelio")
	if err := os.WriteFile(runPath, []byte(launcher), 0o755); err != nil {
		return "", err
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>CFBundleName</key><string>Vortelio</string>
  <key>CFBundleDisplayName</key><string>Vortelio</string>
  <key>CFBundleIdentifier</key><string>app.vortelio.gui</string>
  <key>CFBundleVersion</key><string>` + version.Version + `</string>
  <key>CFBundleExecutable</key><string>Vortelio</string>
  <key>CFBundleIconFile</key><string>vortelio.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
`
	if err := os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	return app, nil
}

// createLinuxDesktop writes a freedesktop .desktop entry so Vortelio appears in
// the application menu / search.
func createLinuxDesktop(exe string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ico := iconPath()
	desktop := strings.Join([]string{
		"[Desktop Entry]",
		"Type=Application",
		"Name=Vortelio",
		"Comment=AI locale — apri la GUI",
		"Exec=" + exe + " gui",
		"Icon=" + ico,
		"Terminal=false",
		"Categories=Development;Utility;",
		"",
	}, "\n")
	p := filepath.Join(dir, "vortelio.desktop")
	if err := os.WriteFile(p, []byte(desktop), 0o644); err != nil {
		return "", err
	}
	return p, nil
}

// shQuote single-quotes a string for POSIX shells.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
