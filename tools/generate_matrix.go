package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Group struct {
	Name     string   `json:"name"`
	Packages []string `json:"packages"`
}

type Matrix struct {
	Include []Group `json:"include"`
}

func main() {
	cmd := exec.Command("go", "list", "./...")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
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
		panic(err)
	}

	numGroups := 4
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

	jsonMatrix, err := json.Marshal(matrix)
	if err != nil {
		panic(err)
	}

	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		// Fallback for local testing
		_, _ = fmt.Println(string(jsonMatrix))
		return
	}
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("matrix=%s\n", string(jsonMatrix))); err != nil {
		panic(err)
	}
}
