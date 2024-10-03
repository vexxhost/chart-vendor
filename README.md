# `chart-vendor`

## Overview
This is a simple tool which allows you to vendor Helm charts from external
sources into your repository, with the ability to apply patches to the vendored
charts.

## Requirements

- Go 1.23 or newer

## Installation

First, clone the repository and build the binary:

```bash
Copy code
git clone https://github.com/vexxhost/chart-vendor.git
cd chart-vendor
go build -o chart-vendor .
```

## Usage

### Command-line Flags

- `--config-file`: Specifies the configuration file for charts. Default is .charts.yml.
- `--charts-root`: Specifies the root path where charts are stored. Default is charts.
- `--check`: Optionally checks for uncommitted changes or untracked files.

### Example Commands

- Fetch Charts
Fetch and manage vendored charts as specified in the configuration file:

```bash
./chart-vendor --config-file .charts.yml --charts-root ./charts
```

- Check for Uncommitted Changes
Run a check to ensure no changes are left uncommitted in the charts:

```bash
./chart-vendor --check
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
