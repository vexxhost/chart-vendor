# `chart-vendor`

## Overview
This is a simple tool which allows you to vendor Helm charts from external
sources into your repository, with the ability to apply patches to the vendored
charts.

## Installation
```sh
pip install chart-vendor
```

## Usage
### Basic Command
```sh
chart-vendor [CHART_NAME]
```
- `CHART_NAME` (optional): The name of the specific chart to fetch. If omitted,
all charts specified in the configuration will be fetched.

### Options
- `--charts-root`: Root path where charts are generated. Default is `charts`.
- `--check`: Check if all chart manifests are applied or not. If there are
uncommitted changes or untracked files, the check will fail.

## Examples
### Fetch All Charts
```sh
chart-vendor
```
### Fetch a Specific Chart
```sh
chart-vendor my-chart
```
### Check for Uncommitted Changes
```sh
chart-vendor --check
```

## Configuration
The CLI expects a configuration file named `.charts.yml` in the current working
directory. This file should define the charts to be managed. The format of the
configuration file is based on Pydantic models.

**Example Configuration**
```yaml
charts:
  - name: my-chart
    repository:
      url: https://example.com/charts
    version: 1.0.0
    dependencies:
      - name: dependency-chart
        repository:
          url: https://example.com/dependency-charts
        version: 1.2.3
    patches:
      gerrit:
        example.gerrit.com:
          - 12345
          - 67890
```
