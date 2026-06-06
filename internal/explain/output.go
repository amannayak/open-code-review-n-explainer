package explain

import (
	"encoding/json"
	"fmt"
	"os"
)

// PrintText renders the explanation in terminal-friendly Markdown.
func PrintText(result *Result) {
	fmt.Println("\n# Code Explanation")
	if result.Summary != "" {
		fmt.Println("\n" + result.Summary)
	}
	if result.Architecture != "" {
		fmt.Println("\n## Connected Architecture")
		fmt.Println()
		fmt.Println(result.Architecture)
	}
	if len(result.Files) > 0 {
		fmt.Println("\n## File-By-File")
		for _, file := range result.Files {
			fmt.Printf("\n### %s\n", file.Path)
			if file.Purpose != "" {
				fmt.Println("\n" + file.Purpose)
			}
			if file.Explanation != "" {
				fmt.Println("\n" + file.Explanation)
			}
		}
	}
	if len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			fmt.Fprintf(os.Stderr, "[ocr] WARNING explain: %s\n", warning)
		}
	}
}

// PrintJSON writes structured explain output.
func PrintJSON(result *Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
