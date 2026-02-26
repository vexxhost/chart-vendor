package filterdiff

import (
	"strings"
	"testing"

	"github.com/sourcegraph/go-diff/diff"
)

func TestStripPathComponents(t *testing.T) {
	tests := []struct {
		name string
		path string
		n    int
		want string
	}{
		{
			name: "strip one component",
			path: "a/chart/file.yaml",
			n:    1,
			want: "chart/file.yaml",
		},
		{
			name: "strip two components",
			path: "a/b/chart/file.yaml",
			n:    2,
			want: "chart/file.yaml",
		},
		{
			name: "strip zero components",
			path: "chart/file.yaml",
			n:    0,
			want: "chart/file.yaml",
		},
		{
			name: "strip more than depth",
			path: "a/b",
			n:    5,
			want: "b",
		},
		{
			name: "no slashes",
			path: "file.yaml",
			n:    1,
			want: "file.yaml",
		},
		{
			name: "empty string",
			path: "",
			n:    1,
			want: "",
		},
		{
			name: "trailing slash",
			path: "a/b/",
			n:    1,
			want: "b/",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripPathComponents(tt.path, tt.n)
			if got != tt.want {
				t.Errorf("StripPathComponents(%q, %d) = %q, want %q", tt.path, tt.n, got, tt.want)
			}
		})
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		input   string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "chart/values.yaml",
			input:   "chart/values.yaml",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "chart/values.yaml",
			input:   "chart/other.yaml",
			want:    false,
		},
		{
			name:    "star matches filename",
			pattern: "chart/*",
			input:   "chart/values.yaml",
			want:    true,
		},
		{
			name:    "star matches across separators",
			pattern: "chart/*",
			input:   "chart/templates/deployment.yaml",
			want:    true,
		},
		{
			name:    "star does not match different prefix",
			pattern: "chart/*",
			input:   "other/values.yaml",
			want:    false,
		},
		{
			name:    "question mark matches single char",
			pattern: "chart/value?.yaml",
			input:   "chart/values.yaml",
			want:    true,
		},
		{
			name:    "question mark does not match empty",
			pattern: "chart/value?.yaml",
			input:   "chart/value.yaml",
			want:    false,
		},
		{
			name:    "multiple stars",
			pattern: "*/*",
			input:   "chart/values.yaml",
			want:    true,
		},
		{
			name:    "star at start",
			pattern: "*.yaml",
			input:   "chart/templates/deployment.yaml",
			want:    true,
		},
		{
			name:    "empty pattern empty name",
			pattern: "",
			input:   "",
			want:    true,
		},
		{
			name:    "empty pattern non-empty name",
			pattern: "",
			input:   "file",
			want:    false,
		},
		{
			name:    "star matches empty",
			pattern: "*",
			input:   "",
			want:    true,
		},
		{
			name:    "values_overrides wildcard",
			pattern: "chart/values_overrides/*",
			input:   "chart/values_overrides/prod.yaml",
			want:    true,
		},
		{
			name:    "values_overrides wildcard no match on parent",
			pattern: "chart/values_overrides/*",
			input:   "chart/values.yaml",
			want:    false,
		},
		{
			name:    "exact Chart.yaml match",
			pattern: "chart/Chart.yaml",
			input:   "chart/Chart.yaml",
			want:    true,
		},
		{
			name:    "Chart.yaml pattern doesn't match values.yaml",
			pattern: "chart/Chart.yaml",
			input:   "chart/values.yaml",
			want:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchWildcard(tt.pattern, tt.input)
			if got != tt.want {
				t.Errorf("MatchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.want)
			}
		})
	}
}

func TestBestFilename(t *testing.T) {
	tests := []struct {
		name     string
		origName string
		newName  string
		want     string
	}{
		{
			name:     "normal modification",
			origName: "a/chart/values.yaml",
			newName:  "b/chart/values.yaml",
			want:     "a/chart/values.yaml",
		},
		{
			name:     "new file",
			origName: "/dev/null",
			newName:  "b/chart/new_file.yaml",
			want:     "b/chart/new_file.yaml",
		},
		{
			name:     "deleted file",
			origName: "a/chart/old_file.yaml",
			newName:  "/dev/null",
			want:     "a/chart/old_file.yaml",
		},
		{
			name:     "empty orig",
			origName: "",
			newName:  "b/chart/file.yaml",
			want:     "b/chart/file.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd := &diff.FileDiff{
				OrigName: tt.origName,
				NewName:  tt.newName,
			}
			got := bestFilename(fd)
			if got != tt.want {
				t.Errorf("bestFilename() = %q, want %q", got, tt.want)
			}
		})
	}
}

const sampleDiff = `diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1,2 +1,3 @@
 key: value
+newkey: newvalue
 other: other
diff --git a/chart/Chart.yaml b/chart/Chart.yaml
--- a/chart/Chart.yaml
+++ b/chart/Chart.yaml
@@ -1,2 +1,2 @@
-name: chart
+name: chart-updated
 version: 1.0.0
diff --git a/other-chart/values.yaml b/other-chart/values.yaml
--- a/other-chart/values.yaml
+++ b/other-chart/values.yaml
@@ -1 +1 @@
-old: value
+new: value
diff --git a/chart/templates/deployment.yaml b/chart/templates/deployment.yaml
--- a/chart/templates/deployment.yaml
+++ b/chart/templates/deployment.yaml
@@ -1 +1,2 @@
 kind: Deployment
+  name: test
`

func TestFilterIncludesOnly(t *testing.T) {
	result, err := Filter(sampleDiff, 1, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
	if !strings.Contains(result, "chart/Chart.yaml") {
		t.Error("expected chart/Chart.yaml to be included")
	}
	if !strings.Contains(result, "chart/templates/deployment.yaml") {
		t.Error("expected chart/templates/deployment.yaml to be included")
	}
	if strings.Contains(result, "other-chart") {
		t.Error("expected other-chart to be excluded")
	}
}

func TestFilterExcludesChartYaml(t *testing.T) {
	includes := []string{"chart/*"}
	excludes := []string{"chart/Chart.yaml"}

	result, err := Filter(sampleDiff, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
	if strings.Contains(result, "chart/Chart.yaml") {
		t.Error("expected chart/Chart.yaml to be excluded")
	}
	if !strings.Contains(result, "chart/templates/deployment.yaml") {
		t.Error("expected chart/templates/deployment.yaml to be included")
	}
	if strings.Contains(result, "other-chart") {
		t.Error("expected other-chart to be excluded")
	}
}

func TestFilterExcludesValuesOverrides(t *testing.T) {
	input := `diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1 +1,2 @@
 key: value
+new: value
diff --git a/chart/values_overrides/prod.yaml b/chart/values_overrides/prod.yaml
--- a/chart/values_overrides/prod.yaml
+++ b/chart/values_overrides/prod.yaml
@@ -1 +1 @@
-old: value
+new: value
`
	includes := []string{"chart/*"}
	excludes := []string{"chart/values_overrides/*"}

	result, err := Filter(input, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
	if strings.Contains(result, "values_overrides") {
		t.Error("expected chart/values_overrides/prod.yaml to be excluded")
	}
}

func TestFilterMatchesPatchBehavior(t *testing.T) {
	// This test validates the exact behavior expected by Patch():
	// include chart/* but exclude chart/Chart.yaml and chart/values_overrides/*
	includes := []string{"chart/*"}
	excludes := []string{"chart/Chart.yaml", "chart/values_overrides/*"}

	result, err := Filter(sampleDiff, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	// Should include values.yaml and templates/deployment.yaml
	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
	if !strings.Contains(result, "chart/templates/deployment.yaml") {
		t.Error("expected chart/templates/deployment.yaml to be included")
	}

	// Should exclude Chart.yaml
	if strings.Contains(result, "chart/Chart.yaml") {
		t.Error("expected chart/Chart.yaml to be excluded")
	}

	// Should exclude other-chart
	if strings.Contains(result, "other-chart") {
		t.Error("expected other-chart to be excluded")
	}
}

func TestFilterNewFile(t *testing.T) {
	input := `diff --git a/chart/new_file.yaml b/chart/new_file.yaml
new file mode 100644
--- /dev/null
+++ b/chart/new_file.yaml
@@ -0,0 +1,2 @@
+key: value
+other: value
`
	result, err := Filter(input, 1, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/new_file.yaml") {
		t.Error("expected new file to be included")
	}
}

func TestFilterDeletedFile(t *testing.T) {
	input := `diff --git a/chart/old_file.yaml b/chart/old_file.yaml
deleted file mode 100644
--- a/chart/old_file.yaml
+++ /dev/null
@@ -1,2 +0,0 @@
-key: value
-other: value
`
	result, err := Filter(input, 1, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/old_file.yaml") {
		t.Error("expected deleted file to be included")
	}
}

func TestFilterEmptyInput(t *testing.T) {
	result, err := Filter("", 0, []string{"*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result for empty input, got %q", result)
	}
}

func TestFilterNoMatch(t *testing.T) {
	result, err := Filter(sampleDiff, 1, []string{"nonexistent/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result when nothing matches, got %q", result)
	}
}

func TestFilterStripComponents(t *testing.T) {
	input := `diff --git a/prefix/chart/values.yaml b/prefix/chart/values.yaml
--- a/prefix/chart/values.yaml
+++ b/prefix/chart/values.yaml
@@ -1 +1,2 @@
 key: value
+new: value
`
	// Strip 2 components (a/ + prefix/) to get chart/values.yaml
	result, err := Filter(input, 2, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected file to be included after stripping 2 components")
	}
}

func TestFilterMultipleExcludes(t *testing.T) {
	input := `diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1 +1,2 @@
 key: value
+new: value
diff --git a/chart/Chart.yaml b/chart/Chart.yaml
--- a/chart/Chart.yaml
+++ b/chart/Chart.yaml
@@ -1 +1 @@
-old: val
+new: val
diff --git a/chart/values_overrides/prod.yaml b/chart/values_overrides/prod.yaml
--- a/chart/values_overrides/prod.yaml
+++ b/chart/values_overrides/prod.yaml
@@ -1 +1 @@
-old: val
+new: val
diff --git a/chart/templates/svc.yaml b/chart/templates/svc.yaml
--- a/chart/templates/svc.yaml
+++ b/chart/templates/svc.yaml
@@ -1 +1,2 @@
 kind: Service
+  name: svc
`
	includes := []string{"chart/*"}
	excludes := []string{"chart/Chart.yaml", "chart/values_overrides/*"}

	result, err := Filter(input, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
	if !strings.Contains(result, "chart/templates/svc.yaml") {
		t.Error("expected chart/templates/svc.yaml to be included")
	}
	if strings.Contains(result, "chart/Chart.yaml") {
		t.Error("expected chart/Chart.yaml to be excluded")
	}
	if strings.Contains(result, "values_overrides") {
		t.Error("expected chart/values_overrides/prod.yaml to be excluded")
	}
}

func TestFilterPreservesExtendedHeaders(t *testing.T) {
	input := `diff --git a/chart/new.yaml b/chart/new.yaml
new file mode 100644
--- /dev/null
+++ b/chart/new.yaml
@@ -0,0 +1 @@
+content: here
`
	result, err := Filter(input, 1, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "new file mode") {
		t.Error("expected extended header 'new file mode' to be preserved")
	}
}

func TestFilterRenamedFile(t *testing.T) {
	input := `diff --git a/chart/old_name.yaml b/chart/new_name.yaml
rename from chart/old_name.yaml
rename to chart/new_name.yaml
--- a/chart/old_name.yaml
+++ b/chart/new_name.yaml
@@ -1 +1 @@
-old: value
+new: value
`
	result, err := Filter(input, 1, []string{"chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/old_name.yaml") || !strings.Contains(result, "chart/new_name.yaml") {
		t.Error("expected renamed file to be included")
	}
}

func TestFilterExcludeTakesPrecedence(t *testing.T) {
	// If a file matches both include and exclude, exclude wins
	input := `diff --git a/chart/Chart.yaml b/chart/Chart.yaml
--- a/chart/Chart.yaml
+++ b/chart/Chart.yaml
@@ -1 +1 @@
-old: val
+new: val
`
	includes := []string{"chart/*", "chart/Chart.yaml"}
	excludes := []string{"chart/Chart.yaml"}

	result, err := Filter(input, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if strings.Contains(result, "Chart.yaml") {
		t.Error("expected exclude to take precedence over include")
	}
}

func TestFilterGerritPatchFormat(t *testing.T) {
	// Gerrit patches include a commit message header before the diff
	input := `From abc123456789 Mon Sep 17 00:00:00 2001
From: Author <author@example.com>
Date: Mon, 1 Jan 2024 00:00:00 +0000
Subject: [PATCH] Fix chart values

Some commit message body here.
---
 chart/values.yaml | 1 +
 1 file changed, 1 insertion(+)

diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1 +1,2 @@
 key: value
+new: value
diff --git a/chart/Chart.yaml b/chart/Chart.yaml
--- a/chart/Chart.yaml
+++ b/chart/Chart.yaml
@@ -1 +1 @@
-name: old
+name: new
`
	includes := []string{"chart/*"}
	excludes := []string{"chart/Chart.yaml"}

	result, err := Filter(input, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included from gerrit-style patch")
	}
	if strings.Contains(result, "chart/Chart.yaml") {
		t.Error("expected chart/Chart.yaml to be excluded from gerrit-style patch")
	}
}

func TestFilterZeroStripComponents(t *testing.T) {
	input := `diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1 +1,2 @@
 key: value
+new: value
`
	// With strip 0, the full path including a/ prefix is used for matching
	result, err := Filter(input, 0, []string{"a/chart/*"}, nil)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected file to be included with strip 0 and a/ prefix pattern")
	}
}

func TestFilterDeepNestedExclude(t *testing.T) {
	input := `diff --git a/chart/templates/nested/deep/file.yaml b/chart/templates/nested/deep/file.yaml
--- a/chart/templates/nested/deep/file.yaml
+++ b/chart/templates/nested/deep/file.yaml
@@ -1 +1 @@
-old: val
+new: val
diff --git a/chart/values.yaml b/chart/values.yaml
--- a/chart/values.yaml
+++ b/chart/values.yaml
@@ -1 +1 @@
-old: val
+new: val
`
	includes := []string{"chart/*"}
	excludes := []string{"chart/templates/nested/*"}

	result, err := Filter(input, 1, includes, excludes)
	if err != nil {
		t.Fatalf("Filter() error = %v", err)
	}

	if strings.Contains(result, "nested/deep") {
		t.Error("expected deeply nested file to be excluded")
	}
	if !strings.Contains(result, "chart/values.yaml") {
		t.Error("expected chart/values.yaml to be included")
	}
}
