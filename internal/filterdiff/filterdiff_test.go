package filterdiff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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

// runFilterdiff runs the real filterdiff binary with the given parameters and
// returns its output. This mirrors the original Patch() pipeline:
// filterdiff -p1 -I <includes> | filterdiff -p1 -X <excludes>
func runFilterdiff(t *testing.T, input string, includes, excludes []string) string {
	t.Helper()

	if _, err := exec.LookPath("filterdiff"); err != nil {
		t.Skip("filterdiff (patchutils) not installed, skipping comparison test")
	}

	// Write include patterns to temp file
	includeFile, err := os.CreateTemp(t.TempDir(), "includes")
	if err != nil {
		t.Fatal(err)
	}
	_, err = includeFile.WriteString(strings.Join(includes, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	includeFile.Close()

	// Write exclude patterns to temp file
	excludeFile, err := os.CreateTemp(t.TempDir(), "excludes")
	if err != nil {
		t.Fatal(err)
	}
	_, err = excludeFile.WriteString(strings.Join(excludes, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	excludeFile.Close()

	// Run: filterdiff -p1 -I includes | filterdiff -p1 -X excludes
	includeCmd := exec.Command("filterdiff", "-p1", "-I", includeFile.Name())
	includeCmd.Stdin = strings.NewReader(input)

	var includeBuf bytes.Buffer
	includeCmd.Stdout = &includeBuf
	if err := includeCmd.Run(); err != nil {
		// filterdiff returns exit 1 if no matches; that's OK
		if includeCmd.ProcessState.ExitCode() != 1 {
			t.Fatalf("filterdiff include failed: %v", err)
		}
	}

	excludeCmd := exec.Command("filterdiff", "-p1", "-X", excludeFile.Name())
	excludeCmd.Stdin = &includeBuf

	var excludeBuf bytes.Buffer
	excludeCmd.Stdout = &excludeBuf
	if err := excludeCmd.Run(); err != nil {
		if excludeCmd.ProcessState.ExitCode() != 1 {
			t.Fatalf("filterdiff exclude failed: %v", err)
		}
	}

	return excludeBuf.String()
}

// extractFilenames returns the set of filenames present in a unified diff
// by looking at "--- " and "+++ " lines. Returns a deduplicated list.
func extractFilenames(diffOutput string) []string {
	seen := make(map[string]bool)
	for _, line := range strings.Split(diffOutput, "\n") {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 && parts[1] != "/dev/null" {
				seen[parts[1]] = true
			}
		}
	}
	var names []string
	for name := range seen {
		names = append(names, name)
	}
	return names
}

// TestFilterMatchesFilterdiffOnRealPatches tests that our Filter function
// produces the same file selection as the real filterdiff binary using
// actual patches from the atmosphere project.
func TestFilterMatchesFilterdiffOnRealPatches(t *testing.T) {
	tests := []struct {
		name      string
		chartName string
		input     string
	}{
		{
			name:      "multi-chart patch with deleted file",
			chartName: "heat",
			input: `From a7f921d10b9a52c2ed29519e0e97c9931150698b Mon Sep 17 00:00:00 2001
From: Vladimir Kozhukalov <kozhukalov@gmail.com>
Date: Wed, 30 Apr 2025 18:26:35 -0500
Subject: [PATCH] [htk] job_ks_user to create multiple users
---
 heat/templates/job-ks-user-trustee.yaml       | 31 ----------------
 heat/templates/job-ks-user.yaml               |  2 +-
 heat/values.yaml                              |  7 ----
 .../templates/manifests/_job-ks-user.yaml.tpl | 36 +++++++------------
 releasenotes/notes/heat-5e861ec1ee8e2784.yaml |  7 ++++
 6 files changed, 25 insertions(+), 63 deletions(-)

diff --git a/heat/templates/job-ks-user-trustee.yaml b/heat/templates/job-ks-user-trustee.yaml
deleted file mode 100644
index 665be8171..000000000
--- a/heat/templates/job-ks-user-trustee.yaml
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
diff --git a/heat/templates/job-ks-user.yaml b/heat/templates/job-ks-user.yaml
index c5be1fea9..e3fdd6b07 100644
--- a/heat/templates/job-ks-user.yaml
+++ b/heat/templates/job-ks-user.yaml
@@ -1,3 +1,3 @@
 {{- end }}
-{{- $ksUserJob := dict "envAll" . "serviceName" "heat" -}}
+{{- $ksUserJob := dict "envAll" . "serviceName" "heat" "serviceUsers" (tuple "heat" "heat_trustee") -}}
 {{- end -}}
diff --git a/heat/values.yaml b/heat/values.yaml
index 9d33c6476..5075c7e0f 100644
--- a/heat/values.yaml
+++ b/heat/values.yaml
@@ -1,6 +1,5 @@
 dependencies:
         - heat-db-sync
         - heat-rabbit-init
         - heat-ks-user
-        - heat-trustee-ks-user
         - heat-domain-ks-user
diff --git a/helm-toolkit/templates/manifests/_job-ks-user.yaml.tpl b/helm-toolkit/templates/manifests/_job-ks-user.yaml.tpl
index abc123..def456 100644
--- a/helm-toolkit/templates/manifests/_job-ks-user.yaml.tpl
+++ b/helm-toolkit/templates/manifests/_job-ks-user.yaml.tpl
@@ -1,5 +1,3 @@
 line1
-line2
-line3
+line4
 line5
diff --git a/releasenotes/notes/heat-5e861ec1ee8e2784.yaml b/releasenotes/notes/heat-5e861ec1ee8e2784.yaml
new file mode 100644
index 000000000..abc123456
--- /dev/null
+++ b/releasenotes/notes/heat-5e861ec1ee8e2784.yaml
@@ -0,0 +1,3 @@
+---
+features:
+  - some feature
`,
		},
		{
			name:      "simple single-file patch",
			chartName: "nova",
			input: `From f2940941f44ee41bc631941ea5fc316ac8b8253b Mon Sep 17 00:00:00 2001
From: Dong Ma <dong.ma@vexxhost.com>
Date: Tue, 11 Feb 2025 15:19:31 +0000
Subject: [PATCH] Resolve two redundant securityContext problems
---
 nova/templates/statefulset-compute-ironic.yaml | 2 --
 1 file changed, 2 deletions(-)

diff --git a/nova/templates/statefulset-compute-ironic.yaml b/nova/templates/statefulset-compute-ironic.yaml
index 377555d6..37d3fc5a 100644
--- a/nova/templates/statefulset-compute-ironic.yaml
+++ b/nova/templates/statefulset-compute-ironic.yaml
@@ -51,8 +51,6 @@ spec:
 {{ tuple $envAll "nova" "compute-ironic" | include "helm-toolkit.snippets.kubernetes_pod_anti_affinity" | indent 8 }}
       nodeSelector:
         {{ .Values.labels.agent.compute_ironic.node_selector_key }}: {{ .Values.labels.agent.compute_ironic.node_selector_value }}
-      securityContext:
-        runAsUser: 0
       hostPID: true
`,
		},
		{
			name:      "simple values.yaml patch",
			chartName: "horizon",
			input: `diff --git a/horizon/values.yaml b/horizon/values.yaml
index 8cfaf5b4..36de4ee3 100644
--- a/horizon/values.yaml
+++ b/horizon/values.yaml
@@ -385,7 +385,7 @@ conf:
 
         CACHES = {
             'default': {
-                'BACKEND': 'django.core.cache.backends.memcached.MemcachedCache',
+                'BACKEND': 'django.core.cache.backends.memcached.PyMemcacheCache',
                 'LOCATION': '{{ tuple "oslo_cache" "internal" "memcache" . | include "helm-toolkit.endpoints.host_and_port_endpoint_uri_lookup" }}',
             }
         }
`,
		},
		{
			name:      "patch with Chart.yaml and values_overrides that should be excluded",
			chartName: "barbican",
			input: `diff --git a/barbican/Chart.yaml b/barbican/Chart.yaml
index abc123..def456 100644
--- a/barbican/Chart.yaml
+++ b/barbican/Chart.yaml
@@ -1,3 +1,3 @@
 apiVersion: v2
-name: barbican
+name: barbican-modified
 version: 0.1.0
diff --git a/barbican/templates/deployment.yaml b/barbican/templates/deployment.yaml
index abc123..def456 100644
--- a/barbican/templates/deployment.yaml
+++ b/barbican/templates/deployment.yaml
@@ -1,3 +1,4 @@
 apiVersion: apps/v1
 kind: Deployment
+  name: barbican
 metadata:
diff --git a/barbican/values_overrides/prod.yaml b/barbican/values_overrides/prod.yaml
index abc123..def456 100644
--- a/barbican/values_overrides/prod.yaml
+++ b/barbican/values_overrides/prod.yaml
@@ -1 +1,2 @@
 key: value
+extra: override
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			includes := []string{
				fmt.Sprintf("%s/*", tt.chartName),
			}
			excludes := []string{
				fmt.Sprintf("%s/Chart.yaml", tt.chartName),
				fmt.Sprintf("%s/values_overrides/*", tt.chartName),
			}

			// Get our implementation's result
			goResult, err := Filter(tt.input, 1, includes, excludes)
			if err != nil {
				t.Fatalf("Filter() error = %v", err)
			}

			// Get real filterdiff's result
			fdResult := runFilterdiff(t, tt.input, includes, excludes)

			// Compare the filenames present in each output
			goFiles := extractFilenames(goResult)
			fdFiles := extractFilenames(fdResult)

			// Build maps for comparison
			goFileSet := make(map[string]bool)
			for _, f := range goFiles {
				goFileSet[f] = true
			}
			fdFileSet := make(map[string]bool)
			for _, f := range fdFiles {
				fdFileSet[f] = true
			}

			// Check that filterdiff's files are a subset of our files
			for _, f := range fdFiles {
				if !goFileSet[f] {
					t.Errorf("filterdiff included %q but our Filter did not", f)
				}
			}

			// Check that our files are a subset of filterdiff's files
			for _, f := range goFiles {
				if !fdFileSet[f] {
					t.Errorf("our Filter included %q but filterdiff did not", f)
				}
			}

			// Verify both outputs are non-empty when expected
			if fdResult != "" && goResult == "" {
				t.Error("filterdiff produced output but our Filter returned empty")
			}
			if fdResult == "" && goResult != "" {
				t.Error("filterdiff returned empty but our Filter produced output")
			}
		})
	}
}
