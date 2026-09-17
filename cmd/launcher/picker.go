package main

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func pickDirectory(initial string) (string, error) {
	switch runtime.GOOS {
	case "windows":
		script := `$ErrorActionPreference='Stop'; Add-Type -AssemblyName System.Windows.Forms; $d=New-Object System.Windows.Forms.FolderBrowserDialog; $d.Description='Choose a project folder'; $d.ShowNewFolderButton=$true;` +
			` if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::Out.Write($d.SelectedPath) }`
		out, err := exec.Command("powershell.exe", "-NoProfile", "-STA", "-Command", script).Output()
		if err != nil {
			return "", fmt.Errorf("folder picker failed: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	case "darwin":
		out, err := exec.Command("osascript", "-e", `POSIX path of (choose folder with prompt "Choose a project folder")`).Output()
		if err != nil {
			return "", fmt.Errorf("folder picker failed: %w", err)
		}
		return strings.TrimSpace(string(out)), nil
	default:
		if zenity, err := exec.LookPath("zenity"); err == nil {
			args := []string{"--file-selection", "--directory", "--title=Choose a project folder"}
			if initial != "" {
				args = append(args, "--filename="+initial+"/")
			}
			out, err := exec.Command(zenity, args...).Output()
			if err == nil {
				return strings.TrimSpace(string(out)), nil
			}
		}
		if kdialog, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(kdialog, "--getexistingdirectory", initial).Output()
			if err == nil {
				return strings.TrimSpace(string(out)), nil
			}
		}
		return "", errors.New("no supported folder picker found; enter the project path manually")
	}
}
