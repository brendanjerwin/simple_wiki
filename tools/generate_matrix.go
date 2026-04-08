package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const numGroups = 4

type Group struct {
	Name     string   `json:"name"`
	Packages []string `json:"packages"`
}

type Matrix struct {
	Include []Group `json:"include"`
}

func listPackages() ([]string, error) {
	goPath, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("go not found: %w", err)
	}

	cmd := exec.Command(goPath, "list", "./...")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start command: %w", err)
	}

	var packages []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		pkg := scanner.Text()
		if !strings.Contains(pkg, "vendor") && !strings.Contains(pkg, "/gen/") {
			packages = append(packages, pkg)
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("command wait: %w", err)
	}

	return packages, nil
}

func buildMatrix(packages []string) Matrix {
	groups := make([][]string, numGroups)
	for i, pkg := range packages {
		groups[i%numGroups] = append(groups[i%numGroups], pkg)
	}

	var matrix Matrix
	for i, groupPackages := range groups {
		if len(groupPackages) > 0 {
			matrix.Include = append(matrix.Include, Group{
				Name:     fmt.Sprintf("group-%d", i+1),
				Packages: groupPackages,
			})
		}
	}

	return matrix
}

func writeOutput(jsonMatrix []byte) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		// Fallback for local testing
		_, _ = fmt.Println(string(jsonMatrix))
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("matrix=%s\n", string(jsonMatrix))); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func main() {
	packages, err := listPackages()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	matrix := buildMatrix(packages)

	jsonMatrix, err := json.Marshal(matrix)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "marshal matrix:", err)
		os.Exit(1)
	}

	if err := writeOutput(jsonMatrix); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
