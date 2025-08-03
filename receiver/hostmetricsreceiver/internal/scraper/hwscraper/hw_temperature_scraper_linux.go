// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hwscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

const (
	defaultHwmonBasePath = "/sys/class/hwmon"
)

// getHwmonBasePath returns the configured hwmon path or default
func (s *temperatureScraper) getHwmonBasePath() string {
	if s.config.HwmonPath != "" {
		return s.config.HwmonPath
	}
	return defaultHwmonBasePath
}

// startTemperatureScraping initializes temperature monitoring on Linux
func (s *temperatureScraper) startTemperatureScraping(ctx context.Context, host component.Host) error {
	hwmonBasePath := s.getHwmonBasePath()
	if _, err := os.Stat(hwmonBasePath); os.IsNotExist(err) {
		s.logger.Warn("hwmon not available", zap.String("path", hwmonBasePath))
		return fmt.Errorf("%w: hwmon not available at %s", ErrHWMonUnavailable, hwmonBasePath)
	}

	// Initial scan for sensors
	if err := s.scanSensors(); err != nil {
		s.logger.Error("Failed to scan sensors", zap.Error(err))
		return err
	}

	s.logger.Info("Temperature scraper started", zap.Int("sensors", len(s.cachedSensors)))
	return nil
}

// scrapeTemperatureMetrics collects temperature metrics on Linux
func (s *temperatureScraper) scrapeTemperatureMetrics(ctx context.Context) error {
	now := time.Now()

	// Rescan sensors periodically if configured
	if s.config.ScanInterval > 0 && now.Sub(s.lastScanTime) > time.Duration(s.config.ScanInterval)*time.Second {
		if err := s.scanSensors(); err != nil {
			s.logger.Error("Failed to rescan sensors", zap.Error(err))
		}
	}

	for _, sensor := range s.cachedSensors {
		if err := s.scrapeTemperatureSensor(sensor, now); err != nil {
			s.logger.Debug("Failed to scrape sensor",
				zap.String("sensor", sensor.ID),
				zap.Error(err))
		}
	}

	return nil
}

// scanSensors discovers available temperature sensors
func (s *temperatureScraper) scanSensors() error {
	s.logger.Debug("Scanning for temperature sensors")

	hwmonBasePath := s.getHwmonBasePath()
	hwmonDirs, err := filepath.Glob(filepath.Join(hwmonBasePath, "hwmon*"))
	if err != nil {
		return fmt.Errorf("failed to scan hwmon directories: %w", err)
	}

	var sensors []temperatureSensor

	for _, hwmonDir := range hwmonDirs {
		deviceSensors, err := s.scanHwmonDevice(hwmonDir)
		if err != nil {
			s.logger.Debug("Failed to scan device", zap.String("device", hwmonDir), zap.Error(err))
			continue
		}
		sensors = append(sensors, deviceSensors...)
	}

	s.cachedSensors = sensors
	s.lastScanTime = time.Now()

	s.logger.Debug("Temperature sensor scan completed",
		zap.Int("sensors_found", len(sensors)))

	return nil
}

// scanHwmonDevice scans a single hwmon device for temperature sensors
func (s *temperatureScraper) scanHwmonDevice(hwmonDir string) ([]temperatureSensor, error) {
	deviceName, err := s.readDeviceName(hwmonDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read device name: %w", err)
	}

	// Check if this device type should be included
	deviceType := s.detectDeviceType(deviceName)
	if !s.shouldIncludeDevice(deviceName, deviceType) {
		return nil, nil
	}

	// Find temperature input files
	tempFiles, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan temperature files: %w", err)
	}

	var sensors []temperatureSensor

	for _, tempFile := range tempFiles {
		sensor, err := s.createTemperatureSensor(hwmonDir, tempFile, deviceName, deviceType)
		if err != nil {
			s.logger.Debug("Failed to create sensor", zap.String("file", tempFile), zap.Error(err))
			continue
		}
		sensors = append(sensors, sensor)
	}

	return sensors, nil
}

// createTemperatureSensor creates a temperature sensor from a temp input file
func (s *temperatureScraper) createTemperatureSensor(hwmonDir, tempFile, deviceName, deviceType string) (temperatureSensor, error) {
	// Extract sensor number from filename (e.g., temp1_input -> 1)
	baseName := filepath.Base(tempFile)
	sensorNum := strings.TrimSuffix(strings.TrimPrefix(baseName, "temp"), "_input")

	// Try to read sensor label
	labelPath := filepath.Join(hwmonDir, fmt.Sprintf("temp%s_label", sensorNum))
	sensorLabel := s.readOptionalFile(labelPath)
	if sensorLabel == "" {
		sensorLabel = fmt.Sprintf("temp%s", sensorNum)
	}

	sensor := temperatureSensor{
		ID:         fmt.Sprintf("%s_%s", deviceName, sensorLabel),
		Name:       fmt.Sprintf("%s %s", deviceName, sensorLabel),
		DeviceType: deviceType,
		Parent:     deviceName,
		Location:   sensorLabel,
		InputPath:  tempFile,
		MaxPath:    filepath.Join(hwmonDir, fmt.Sprintf("temp%s_max", sensorNum)),
		CritPath:   filepath.Join(hwmonDir, fmt.Sprintf("temp%s_crit", sensorNum)),
		LabelPath:  labelPath,
	}

	return sensor, nil
}

// scrapeTemperatureSensor scrapes metrics from a single temperature sensor
func (s *temperatureScraper) scrapeTemperatureSensor(sensor temperatureSensor, timestamp time.Time) error {
	// Read current temperature
	tempStr, err := s.readSensorFile(sensor.InputPath)
	if err != nil {
		return fmt.Errorf("failed to read temperature: %w", err)
	}

	tempMilliCelsius, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse temperature: %w", err)
	}

	// Convert from millidegrees to degrees Celsius
	tempCelsius := tempMilliCelsius / 1000.0

	// Convert to desired units
	temperature := s.convertTemperature(tempCelsius)

	// Record temperature metric
	s.mb.RecordHwTemperatureDataPoint(
		pcommon.NewTimestampFromTime(timestamp),
		temperature,
		sensor.ID,
		sensor.Name,
		sensor.Parent,
		sensor.Location,
	)

	// Record limit metrics if available and enabled
	if s.mb.config.Metrics.HwTemperatureLimit.Enabled {
		s.scrapeLimitMetrics(sensor, timestamp)
	}

	// Record status metric
	s.mb.RecordHwStatusDataPoint(
		pcommon.NewTimestampFromTime(timestamp),
		1, // Assume OK if we can read the sensor
		sensor.ID,
		metadata.AttributeStateOk,
		metadata.AttributeTypeTemperature,
		sensor.Name,
		sensor.Parent,
	)

	return nil
}

// scrapeLimitMetrics scrapes temperature limit metrics
func (s *temperatureScraper) scrapeLimitMetrics(sensor temperatureSensor, timestamp time.Time) {
	// Check for max limit
	if maxTemp, err := s.readOptionalSensorFile(sensor.MaxPath); err == nil {
		maxCelsius := maxTemp / 1000.0
		maxConverted := s.convertTemperature(maxCelsius)

		s.mb.RecordHwTemperatureLimitDataPoint(
			pcommon.NewTimestampFromTime(timestamp),
			maxConverted,
			sensor.ID,
			metadata.AttributeLimitTypeMax,
			sensor.Name,
			sensor.Parent,
			sensor.Location,
		)
	}

	// Check for critical limit
	if critTemp, err := s.readOptionalSensorFile(sensor.CritPath); err == nil {
		critCelsius := critTemp / 1000.0
		critConverted := s.convertTemperature(critCelsius)

		s.mb.RecordHwTemperatureLimitDataPoint(
			pcommon.NewTimestampFromTime(timestamp),
			critConverted,
			sensor.ID,
			metadata.AttributeLimitTypeCritical,
			sensor.Name,
			sensor.Parent,
			sensor.Location,
		)
	}
}

// Helper functions

func (s *temperatureScraper) readDeviceName(hwmonDir string) (string, error) {
	nameFile := filepath.Join(hwmonDir, "name")
	if name := s.readOptionalFile(nameFile); name != "" {
		return name, nil
	}
	return filepath.Base(hwmonDir), nil
}

func (s *temperatureScraper) detectDeviceType(name string) string {
	return detectDeviceTypeFromName(name)
}

func (s *temperatureScraper) shouldIncludeDevice(deviceName, deviceType string) bool {
	// Log unknown device types for debugging
	if deviceType == DeviceTypeUnknown {
		s.logger.Debug("Unknown device type detected",
			zap.String("device_name", deviceName),
			zap.String("device_type", deviceType))
	}

	// TODO: Implement include/exclude name patterns
	// For now, include all devices that pass the type filter
	if len(s.config.Devices.Types) == 0 {
		// Include unknown devices by default when no type filter is configured
		return true
	}

	for _, allowedType := range s.config.Devices.Types {
		if deviceType == allowedType {
			return true
		}
	}

	// Exclude if device type doesn't match any allowed types
	s.logger.Debug("Device excluded by type filter",
		zap.String("device_name", deviceName),
		zap.String("device_type", deviceType),
		zap.Strings("allowed_types", s.config.Devices.Types))

	return false
}

func (s *temperatureScraper) readSensorFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *temperatureScraper) readOptionalFile(path string) string {
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

func (s *temperatureScraper) readOptionalSensorFile(path string) (float64, error) {
	content := s.readOptionalFile(path)
	if content == "" {
		return 0, fmt.Errorf("file not found or empty")
	}
	return strconv.ParseFloat(content, 64)
}

// detectDeviceTypeFromName attempts to determine a device type from name
func detectDeviceTypeFromName(name string) string {
	lowerName := strings.ToLower(name)

	// Check each device type pattern
	if containsAny(lowerName, cpuPatterns) {
		return DeviceTypeCPU
	}
	if containsAny(lowerName, gpuPatterns) {
		return DeviceTypeGPU
	}
	if containsAny(lowerName, storagePatterns) {
		return DeviceTypeStorage
	}
	if containsAny(lowerName, powerPatterns) {
		return DeviceTypePowerSupply
	}
	if containsAny(lowerName, fanPatterns) {
		return DeviceTypeFan
	}
	if containsAny(lowerName, memoryPatterns) {
		return DeviceTypeMemory
	}
	if containsAny(lowerName, motherboardPatterns) {
		return DeviceTypeMotherboard
	}

	// If no pattern matches, return unknown
	return DeviceTypeUnknown
}

// containsAny checks if the input string contains any of the patterns
func containsAny(input string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	return false
}
