package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type operation struct {
	Summary   string         `yaml:"summary"`
	Responses map[string]any `yaml:"responses"`
}

type spec struct {
	OpenAPI string `yaml:"openapi"`
	Info    struct {
		Title   string `yaml:"title"`
		Version string `yaml:"version"`
	} `yaml:"info"`
	Paths map[string]map[string]operation `yaml:"paths"`
}

func main() {
	path := "api/openapi-outline.yaml"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}

	var parsed spec
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		fatal("parse %s: %v", path, err)
	}

	if !strings.HasPrefix(parsed.OpenAPI, "3.1.") {
		fatal("%s: expected openapi 3.1.x, got %q", path, parsed.OpenAPI)
	}
	if strings.TrimSpace(parsed.Info.Title) == "" {
		fatal("%s: info.title must be set", path)
	}
	if strings.TrimSpace(parsed.Info.Version) == "" {
		fatal("%s: info.version must be set", path)
	}
	if len(parsed.Paths) == 0 {
		fatal("%s: paths must not be empty", path)
	}

	for pathName, operations := range parsed.Paths {
		if len(operations) == 0 {
			fatal("%s: path %s has no operations", path, pathName)
		}
		for method, op := range operations {
			if strings.TrimSpace(op.Summary) == "" {
				fatal("%s: %s %s is missing a summary", path, method, pathName)
			}
			if len(op.Responses) == 0 {
				fatal("%s: %s %s has no responses", path, method, pathName)
			}
		}
	}

	fmt.Printf("validated %s (%s)\n", path, parsed.OpenAPI)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "openapi-validate: "+format+"\n", args...)
	os.Exit(1)
}
