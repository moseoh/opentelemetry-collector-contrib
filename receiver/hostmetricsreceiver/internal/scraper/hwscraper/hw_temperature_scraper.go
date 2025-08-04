// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// Temperature validation constants
// A reasonable temperature range for hardware sensors
const (
	MinValidTemperature = -40000.0 // Extreme cold conditions
	MaxValidTemperature = 200000.0 // High-end server/industrial equipment
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
	DeviceType metadata.AttributeType
	Parent     string
	Location   string
	InputPath  string                                 // Temperature value
	LabelPath  string                                 // Temperature sensor name
	LimitPaths map[metadata.AttributeLimitType]string // threshold paths with keys matching temperatureLimitFormats
}

// newTemperatureScraper creates a new temperature scraper
func newTemperatureScraper(logger *zap.Logger, mb *metadata.MetricsBuilder, cfg *Config) *temperatureScraper {
	return &temperatureScraper{
		logger: logger,
		mb:     mb,
		config: cfg,
	}
}

// convertTemperature converts tempMilliCelsius from Celsius to the configured unit
func (s *temperatureScraper) convertTemperature(tempMilliCelsius float64) (float64, error) {
	if !s.isValidTemperature(tempMilliCelsius) {
		return 0, fmt.Errorf("invalid temperature reading: %.2f°C (outside range %.1f-%.1f°C)",
			tempMilliCelsius, MinValidTemperature, MaxValidTemperature)
	}

	switch s.config.TemperatureUnits {
	case "fahrenheit", "f":
		return tempMilliCelsius/1000*9/5 + 32, nil
	case "celsius", "c":
		return tempMilliCelsius / 1000, nil
	default:
		return 0, fmt.Errorf("invalid temperature_units: %s (must be 'celsius' or 'fahrenheit')", s.config.TemperatureUnits)
	}
}

// isValidTemperature performs sanity check on temperature readings
func (s *temperatureScraper) isValidTemperature(tempMilliCelsius float64) bool {
	return tempMilliCelsius >= MinValidTemperature && tempMilliCelsius <= MaxValidTemperature
}
