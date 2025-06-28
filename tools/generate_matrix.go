package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

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

	type Group {
		Name     string   `json:"name"`
		Packages []string `json:"packages"`
	}

	type Matrix {
		Include []Group `json:"include"`
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

	fmt.Printf("::set-output name=matrix::%s\n", string(jsonMatrix))
}
