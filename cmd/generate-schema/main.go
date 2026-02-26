// Command generate-schema generates the JSON schema for the .charts.yml
// configuration file from the Go config structs and writes it to
// charts.schema.json at the repository root.
//
// Usage:
//
//	go run ./cmd/generate-schema
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/vexxhost/chart-vendor/internal/config"
)

func main() {
	schema := config.GenerateSchema()

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error marshaling schema: %v\n", err)
		os.Exit(1)
	}
	data = append(data, '\n')

	output := "charts.schema.json"
	if len(os.Args) > 1 {
		output = os.Args[1]
	}

	if err := os.WriteFile(output, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing schema: %v\n", err)
		os.Exit(1)
	}
}
