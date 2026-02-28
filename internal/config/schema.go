package config

import (
	"github.com/invopop/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// GenerateSchema returns the JSON schema for the Config type, derived from
// Go struct reflection. It is the authoritative source for charts.schema.json.
func GenerateSchema() *jsonschema.Schema {
	r := &jsonschema.Reflector{
		Anonymous:      true,
		ExpandedStruct: true,
		CommentMap: map[string]string{
			"helm.sh/helm/v4/pkg/chart/v2.Dependency":              "A Helm chart dependency",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Name":         "Name of the dependency chart. This must match the name in the dependency's Chart.yaml.",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Version":      "Version or version range of the dependency chart",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Repository":   "URL of the repository containing the dependency chart",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Condition":    "A YAML path that resolves to a boolean, used for enabling/disabling the chart (e.g. subchart1.enabled)",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Tags":         "Tags used to group charts for enabling/disabling together",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Enabled":      "Whether the dependency chart should be loaded",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.ImportValues": "Mapping of source values to parent key to be imported",
			"helm.sh/helm/v4/pkg/chart/v2.Dependency.Alias":        "Alias to be used for the dependency chart",
		},
	}

	schema := r.Reflect(&Config{})
	schema.ID = "https://raw.githubusercontent.com/vexxhost/chart-vendor/main/charts.schema.json"
	schema.Title = "chart-vendor configuration"
	schema.Description = "Configuration file for chart-vendor (.charts.yml)"

	// The import-values field is []interface{} in helm's Dependency struct, so the
	// reflector cannot infer a specific schema. Patch it to reflect the actual helm
	// spec: each item is either a string (shorthand) or an object with "child" and
	// "parent" string keys.
	if depDef, ok := schema.Definitions["Dependency"]; ok {
		if importValsProp, ok := depDef.Properties.Get("import-values"); ok {
			childParentProps := orderedmap.New[string, *jsonschema.Schema]()
			childParentProps.Set("child", &jsonschema.Schema{Type: "string"})
			childParentProps.Set("parent", &jsonschema.Schema{Type: "string"})
			importValsProp.Items = &jsonschema.Schema{
				OneOf: []*jsonschema.Schema{
					{Type: "string"},
					{
						Type:                 "object",
						Properties:           childParentProps,
						Required:             []string{"child", "parent"},
						AdditionalProperties: jsonschema.FalseSchema,
					},
				},
			}
		}
	}

	return schema
}
