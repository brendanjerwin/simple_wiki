// Command read-mutation-baseline prints fields from .mutation-baseline.json for shell scripts.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type mutationBaseline struct {
	MinimumPatchEfficacy float64 `json:"minimum_patch_efficacy"`
}

const maxMutationEfficacy = 100

func main() {
	os.Exit(runMutationBaseline(os.Args[1:], os.Stdout, os.Stderr))
}

func runMutationBaseline(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("read-mutation-baseline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	field := flags.String("field", "minimum_patch_efficacy", "baseline field to print")
	if err := flags.Parse(args); err != nil {
		return 1
	}
	if flags.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, errors.New("usage: read-mutation-baseline [-field minimum_patch_efficacy] <baseline.json>"))
		return 1
	}

	baseline, err := readMutationBaseline(flags.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	switch *field {
	case "minimum_patch_efficacy":
		if _, err := fmt.Fprintf(stdout, "%.1f\n", baseline.MinimumPatchEfficacy); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 1
		}
	default:
		_, _ = fmt.Fprintln(stderr, fmt.Errorf("unsupported field %q", *field))
		return 1
	}
	return 0
}

func readMutationBaseline(path string) (*mutationBaseline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read mutation baseline: %w", err)
	}
	var baseline mutationBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("parse mutation baseline: %w", err)
	}
	if baseline.MinimumPatchEfficacy <= 0 || baseline.MinimumPatchEfficacy > maxMutationEfficacy {
		return nil, fmt.Errorf("minimum_patch_efficacy must be greater than 0 and at most 100, got %.1f", baseline.MinimumPatchEfficacy)
	}
	return &baseline, nil
}
