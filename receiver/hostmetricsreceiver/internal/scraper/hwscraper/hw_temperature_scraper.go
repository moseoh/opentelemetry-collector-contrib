// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// Device type constants
const (
	DeviceTypeCPU         = "cpu"
	DeviceTypeGPU         = "gpu"
	DeviceTypeMotherboard = "motherboard"
	DeviceTypeStorage     = "storage"
	DeviceTypePowerSupply = "power_supply"
	DeviceTypeFan         = "fan"
	DeviceTypeMemory      = "memory"
	DeviceTypeUnknown     = "unknown"
)

// Device name patterns for detection
var (
	cpuPatterns         = []string{"cpu", "coretemp", "k10temp", "tctl", "tdie"}
	gpuPatterns         = []string{"gpu", "nvidia", "amdgpu", "radeon", "nouveau"}
	storagePatterns     = []string{"nvme", "sata", "hdd", "ssd", "ata", "scsi"}
	powerPatterns       = []string{"psu", "power", "acpi"}
	fanPatterns         = []string{"fan"}
	memoryPatterns      = []string{"memory", "dimm", "ram"}
	motherboardPatterns = []string{"motherboard", "mainboard", "system", "chassis"}
)

// temperatureScraper scrapes temperature metrics from hardware sensors
type temperatureScraper struct {
	logger        *zap.Logger
	mb            *metadata.MetricsBuilder
	config        *Config
	lastScanTime  time.Time
	cachedSensors []temperatureSensor
}

type temperatureSensor struct {
	ID         string
	Name       string
	DeviceType string
	Parent     string
	Location   string
	InputPath  string
	MaxPath    string
	CritPath   string
	LabelPath  string
}

// newTemperatureScraper creates a new temperature scraper
func newTemperatureScraper(logger *zap.Logger, mb *metadata.MetricsBuilder, cfg *Config) *temperatureScraper {
	return &temperatureScraper{
		logger: logger,
		mb:     mb,
		config: cfg,
	}
}

// convertTemperature converts temperature from Celsius to the configured unit
func (s *temperatureScraper) convertTemperature(celsius float64) float64 {
	switch s.config.TemperatureUnits {
	case "fahrenheit", "f":
		return celsius*9/5 + 32
	default: // celsius
		return celsius
	}
}
