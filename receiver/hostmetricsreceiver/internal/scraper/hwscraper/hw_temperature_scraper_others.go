// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"
	"fmt"
	"runtime"

	"go.opentelemetry.io/collector/component"
)

// startTemperatureScraping initializes temperature monitoring for non-Linux platforms
func (s *temperatureScraper) startTemperatureScraping(ctx context.Context, host component.Host) error {
	s.logger.Warn("Temperature scraper is not supported on " + runtime.GOOS)
	return fmt.Errorf("%w: temperature metrics scraper is not supported on %s", ErrHWMonUnavailable, runtime.GOOS)
}

// scrapeTemperatureMetrics collects temperature metrics for non-Linux platforms
func (s *temperatureScraper) scrapeTemperatureMetrics(ctx context.Context) error {
	s.logger.Warn("Temperature scraper is not supported on " + runtime.GOOS)
	return fmt.Errorf("temperature metrics scraping is not supported on %s", runtime.GOOS)
}
