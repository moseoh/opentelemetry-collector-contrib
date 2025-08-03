// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

var ErrHWMonUnavailable = errors.New("hwmon not available")

const (
	temperatureScraperLen = 1
)

type hwScraper struct {
	logger             *zap.Logger
	mb                 *metadata.MetricsBuilder
	config             *Config
	temperatureScraper *temperatureScraper
}

func newHwScraper(_ context.Context, settings scraper.Settings, cfg *Config) (*hwScraper, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	mb := metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

	scraper := &hwScraper{
		logger: settings.Logger,
		mb:     mb,
		config: cfg,
	}

	scraper.temperatureScraper = newTemperatureScraper(settings.Logger, mb, cfg)

	return scraper, nil
}

func (s *hwScraper) start(ctx context.Context, host component.Host) error {
	s.logger.Info("Starting hw scraper")

	if err := s.temperatureScraper.startTemperatureScraping(ctx, host); err != nil {
		s.logger.Error("Failed to start temperature scraping", zap.Error(err))
		return err
	}

	return nil
}

func (s *hwScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var scrapeErrors scrapererror.ScrapeErrors

	err := s.temperatureScraper.scrapeTemperatureMetrics(ctx)
	if err != nil {
		scrapeErrors.AddPartial(temperatureScraperLen, err)
	}

	return s.mb.Emit(), scrapeErrors.Combine()
}

func validateConfig(cfg *Config) error {
	switch cfg.TemperatureUnits {
	case "celsius", "fahrenheit", "c", "f", "":
	default:
		return fmt.Errorf("invalid temperature_units: %s (must be 'celsius' or 'fahrenheit')", cfg.TemperatureUnits)
	}

	if cfg.ScanInterval < 0 {
		return errors.New("scan_interval cannot be negative")
	}

	return nil
}
