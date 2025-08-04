// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/scraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

// NewFactory creates a new factory for hw scraper.
func NewFactory() scraper.Factory {
	return scraper.NewFactory(
		metadata.Type,
		createDefaultConfig,
		scraper.WithMetrics(createMetricsScraper, metadata.MetricsStability),
	)
}

// createDefaultConfig creates the default configuration for the scraper.
func createDefaultConfig() component.Config {
	return &Config{
		MetricsBuilderConfig:  metadata.DefaultMetricsBuilderConfig(),
		TemperatureUnits:      "celsius",
		EnableDetailedMetrics: false,
		ScanInterval:          300,
		HwmonPath:             "/sys/class/hwmon",
		Devices: DeviceFilters{
			Types: []string{
				metadata.AttributeTypeCpu.String(),
				metadata.AttributeTypeGpu.String(),
				metadata.AttributeTypePhysicalDisk.String(),
			},
		},
	}
}

// createMetricsScraper creates a hw scraper based on provided config.
func createMetricsScraper(
	ctx context.Context,
	settings scraper.Settings,
	config component.Config,
) (scraper.Metrics, error) {
	cfg := config.(*Config)

	hwScraper, err := newHwScraper(ctx, settings, cfg)
	if err != nil {
		return nil, err
	}

	return scraper.NewMetrics(hwScraper.scrape, scraper.WithStart(hwScraper.start))
}
