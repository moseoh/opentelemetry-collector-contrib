# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

### Building and Testing
```bash
# Build the receiver (from project root)
make hostmetricsreceiver

# Run all tests
make gotest

# Run tests for specific component
go test ./receiver/hostmetricsreceiver/...

# Run a single test
go test -run TestSpecificFunction ./receiver/hostmetricsreceiver/internal/scraper/hwscraper

# Generate code (metrics, etc.)
make generate

# Lint the code
make golint

# Check modules and dependencies
make gomodtidy
```

### Code Generation
```bash
# Generate metrics metadata (when modifying metadata.yaml)
make generate-metrics

# Update go.mod dependencies
make gomodtidy
```

## Architecture Overview

### Host Metrics Receiver Structure
The `hostmetricsreceiver` is part of the OpenTelemetry Collector Contrib project and follows a modular scraper-based architecture:

- **Factory (`factory.go`)**: Creates and configures the receiver instance
- **Config (`config.go`)**: Defines configuration structure and validation
- **Receiver (`hostmetrics_receiver.go`)**: Main receiver logic that orchestrates scrapers
- **Scrapers (`internal/scraper/`)**: Individual metric collection modules

### Scraper Architecture
Each scraper in `internal/scraper/` follows a consistent pattern:
- **Config**: Scraper-specific configuration
- **Factory**: Creates scraper instances
- **Scraper**: Main collection logic with `scrape()` method
- **Metadata**: Auto-generated metrics definitions from `metadata.yaml`

### Key Scrapers
- `cpuscraper`: CPU utilization metrics
- `memoryscraper`: Memory usage metrics  
- `filesystemscraper`: Disk usage metrics
- `networkscraper`: Network interface metrics
- `hwscraper`: Hardware temperature metrics (custom implementation)

### Hardware Scraper (hwscraper)
A specialized scraper for hardware temperature monitoring:
- **Linux Implementation**: Uses `/sys/class/hwmon` interface
- **Cross-platform**: Separate implementations per OS
- **Generated Metrics**: Temperature readings from hardware sensors

### Code Generation
The project uses extensive code generation:
- `metadata.yaml` → `generated_metrics.go` (metric definitions)
- Consistent patterns across all scrapers
- Auto-generated test files and documentation

### Development Patterns
- Each scraper is self-contained with its own config, factory, and implementation
- Platform-specific code uses build tags (`//go:build linux`)
- Metrics are defined declaratively in `metadata.yaml` files
- Error handling uses OpenTelemetry's structured error patterns
- Testing includes both unit tests and integration tests with mock data

### Important Files
- `go.mod`: Project is using Go 1.21+ with OpenTelemetry dependencies
- `Makefile`: Contains all build, test, and code generation commands
- `internal/scraper/*/metadata.yaml`: Metric definitions for each scraper
- Platform-specific files use naming like `*_linux.go`, `*_windows.go`