/*
 * Add All Headers
 *
 * Part of MantisDB - High-performance multi-model database.
 * See CONTRIBUTING.md for code standards and comment guidelines.
 */
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	stats := map[string]int{
		"go_added":  0,
		"rs_added":  0,
		"ts_added":  0,
		"go_total":  0,
		"rs_total":  0,
		"ts_total":  0,
	}

	err := filepath.Walk("..", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}

		// Skip unwanted paths
		if strings.Contains(path, "node_modules") ||
			strings.Contains(path, ".git") ||
			strings.Contains(path, "target") ||
			strings.Contains(path, "dist") ||
			strings.Contains(path, "build") {
			return nil
		}

		ext := filepath.Ext(path)
		switch ext {
		case ".go":
			if !strings.Contains(path, "_test.go") {
				stats["go_total"]++
				if !hasComment(path, "/*") {
					if err := addGoComment(path); err == nil {
						stats["go_added"]++
						fmt.Printf("✓ Go: %s\n", path)
					}
				}
			}
		case ".rs":
			stats["rs_total"]++
			if !hasComment(path, "//!") && !hasComment(path, "/*") {
				if err := addRustComment(path); err == nil {
					stats["rs_added"]++
					fmt.Printf("✓ Rust: %s\n", path)
				}
			}
		case ".ts", ".tsx":
			stats["ts_total"]++
			if !hasComment(path, "/*") {
				if err := addTsComment(path); err == nil {
					stats["ts_added"]++
					fmt.Printf("✓ TS: %s\n", path)
				}
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Go:         %d/%d files updated\n", stats["go_added"], stats["go_total"])
	fmt.Printf("Rust:       %d/%d files updated\n", stats["rs_added"], stats["rs_total"])
	fmt.Printf("TypeScript: %d/%d files updated\n", stats["ts_added"], stats["ts_total"])
	fmt.Printf("Total:      %d files updated\n", stats["go_added"]+stats["rs_added"]+stats["ts_added"])
}

func hasComment(path, prefix string) bool {
	file, err := os.Open(path)
	if err != nil {
		return true // Assume it has comment if we can't read
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func addGoComment(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	comment := generateGoComment(path)
	return os.WriteFile(path, []byte(comment+string(content)), 0644)
}

func addRustComment(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	comment := generateRustComment(path)
	return os.WriteFile(path, []byte(comment+string(content)), 0644)
}

func addTsComment(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	comment := generateTsComment(path)
	return os.WriteFile(path, []byte(comment+string(content)), 0644)
}

func generateGoComment(path string) string {
	name := getComponentName(path)
	return fmt.Sprintf(`/*
 * %s
 *
 * Part of MantisDB - High-performance multi-model database.
 * See CONTRIBUTING.md for code standards and comment guidelines.
 */
`, name)
}

func generateRustComment(path string) string {
	name := getComponentName(path)
	return fmt.Sprintf(`//! %s
//!
//! Part of MantisDB - High-performance multi-model database.
//! See CONTRIBUTING.md for code standards and comment guidelines.

`, name)
}

func generateTsComment(path string) string {
	name := getComponentName(path)
	return fmt.Sprintf(`/**
 * %s
 *
 * Part of MantisDB - High-performance multi-model database.
 * See CONTRIBUTING.md for code standards and comment guidelines.
 */

`, name)
}

func getComponentName(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	filename := parts[len(parts)-1]
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	return strings.Title(name)
}
