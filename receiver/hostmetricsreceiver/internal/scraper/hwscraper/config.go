// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// Config relating to HW Sensor Metric Scraper.
type Config struct {
	metadata.MetricsBuilderConfig `mapstructure:",squash"`

	// Sensors specifies sensor filtering options
	Sensors SensorFilters `mapstructure:"sensors"`

	// Devices specifies device filtering options
	Devices DeviceFilters `mapstructure:"devices"`

	// TemperatureUnits specifies the unit for temperature readings (celsius or fahrenheit)
	// Default: celsius
	TemperatureUnits string `mapstructure:"temperature_units"`

	// EnableDetailedMetrics enables additional detailed hardware metrics like fan speed, voltage, power
	// Default: false
	EnableDetailedMetrics bool `mapstructure:"enable_detailed_metrics"`

	// ScanInterval specifies how often to scan for new hardware sensors (in seconds)
	// Set to 0 to disable dynamic scanning. Default: 300 (5 minutes)
	ScanInterval int `mapstructure:"scan_interval"`

	// HwmonPath specifies the path to hwmon directory (for testing purposes)
	// Default: /sys/class/hwmon
	HwmonPath string `mapstructure:"hwmon_path"`
}

// SensorFilters provides options to filter sensors
type SensorFilters struct {
	// Include specifies sensor name patterns to include
	Include MatchConfig `mapstructure:"include"`

	// Exclude specifies sensor name patterns to exclude
	Exclude MatchConfig `mapstructure:"exclude"`
}

// DeviceFilters provides options to filter devices
type DeviceFilters struct {
	// Include specifies device name patterns to include
	Include MatchConfig `mapstructure:"include"`

	// Exclude specifies device name patterns to exclude
	Exclude MatchConfig `mapstructure:"exclude"`

	// Types specify device types to monitor
	Types []string `mapstructure:"types"`
}

// MatchConfig configures sensor/device name matching
type MatchConfig struct {
	// Names specifies exact names or regex patterns to match
	Names []string `mapstructure:"names"`

	// MatchType specifies the type of matching to apply
	MatchType filterset.MatchType `mapstructure:"match_type"`
}
