package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func runSetup(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printSetupUsage()
		return nil
	}
	switch args[0] {
	case "explain":
		return setupExplain()
	default:
		return fmt.Errorf("unknown setup target %q\nAvailable: explain", args[0])
	}
}

func printSetupUsage() {
	fmt.Println(`Optional helper setup.

Usage:
  ocr setup explain

Commands:
  explain      Install Graphify for connected tutor-mode explanations`)
}

func setupExplain() error {
	for _, name := range []string{"graphify", "graphifyy"} {
		if path, err := exec.LookPath(name); err == nil && path != "" {
			fmt.Printf("Graphify is already installed: %s\n", path)
			return nil
		}
	}
	installers := [][]string{
		{"uv", "tool", "install", "graphifyy"},
		{"pipx", "install", "graphifyy"},
	}
	var attempts []string
	for _, installer := range installers {
		bin := installer[0]
		if _, err := exec.LookPath(bin); err != nil {
			attempts = append(attempts, strings.Join(installer, " ")+" (not available)")
			continue
		}
		cmd := exec.Command(installer[0], installer[1:]...)
		output, err := cmd.CombinedOutput()
		if err == nil {
			fmt.Println("Graphify installed.")
			fmt.Print(string(output))
			return nil
		}
		attempts = append(attempts, strings.Join(installer, " ")+": "+strings.TrimSpace(string(output)))
	}
	return fmt.Errorf("could not install Graphify automatically; run `uv tool install graphifyy` manually\n%s", strings.Join(attempts, "\n"))
}
