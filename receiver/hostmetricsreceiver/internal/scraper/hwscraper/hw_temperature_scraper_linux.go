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

// deviceTypePatterns maps device types to their detection patterns
var deviceTypePatterns = map[metadata.AttributeType][]string{
	metadata.AttributeTypeCpu:          {"cpu", "coretemp", "k10temp", "tctl", "tdie"},
	metadata.AttributeTypeGpu:          {"gpu", "nvidia", "amdgpu", "radeon", "nouveau"},
	metadata.AttributeTypePhysicalDisk: {"nvme", "sata", "hdd", "ssd", "ata", "scsi"},
	metadata.AttributeTypePowerSupply:  {"psu", "power", "acpi"},
	metadata.AttributeTypeFan:          {"fan"},
	metadata.AttributeTypeMemory:       {"memory", "dimm", "ram"},
}

// SensorPaths holds all file paths for a temperature sensor
type SensorPaths struct {
	Input   string // temp1_input
	Label   string // temp1_label
	Max     string // temp1_max
	MaxCrit string // temp1_crit
	Min     string // temp1_min
	MinCrit string // temp1_lcrit
}

// generateSensorPaths creates all file paths for a temperature sensor
func (s *temperatureScraper) generateSensorPaths(hwmonDir, sensorIndex string) SensorPaths {
	return SensorPaths{
		Input:   filepath.Join(hwmonDir, fmt.Sprintf("temp%s_input", sensorIndex)),
		Label:   filepath.Join(hwmonDir, fmt.Sprintf("temp%s_label", sensorIndex)),
		Max:     filepath.Join(hwmonDir, fmt.Sprintf("temp%s_max", sensorIndex)),
		MaxCrit: filepath.Join(hwmonDir, fmt.Sprintf("temp%s_crit", sensorIndex)),
		Min:     filepath.Join(hwmonDir, fmt.Sprintf("temp%s_min", sensorIndex)),
		MinCrit: filepath.Join(hwmonDir, fmt.Sprintf("temp%s_lcrit", sensorIndex)),
	}
}

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
	hwmonDeviceName, err := s.readDeviceName(hwmonDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read device name: %w", err)
	}

	// Check if this device type should be included
	hwmonDeviceType := detectDeviceTypeFromName(hwmonDeviceName)
	if !s.shouldIncludeDevice(hwmonDeviceName, hwmonDeviceType) {
		return nil, nil
	}

	// Find temperature input files in hwmon device directory
	tempInputFiles, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan temperature files: %w", err)
	}

	var temperatureSensors []temperatureSensor

	for _, tempInputPath := range tempInputFiles {
		sensor, err := s.createTemperatureSensor(hwmonDir, tempInputPath, hwmonDeviceName, hwmonDeviceType)
		if err != nil {
			s.logger.Debug("Failed to create sensor", zap.String("file", tempInputPath), zap.Error(err))
			continue
		}
		temperatureSensors = append(temperatureSensors, sensor)
	}

	return temperatureSensors, nil
}

// createTemperatureSensor creates a temperature sensor from a temp input file
func (s *temperatureScraper) createTemperatureSensor(hwmonDir, inputPath, deviceName string, deviceType metadata.AttributeType) (temperatureSensor, error) {
	// Extract sensor index from filename (e.g., temp1_input -> 1)
	filename := filepath.Base(inputPath)
	sensorIndex := s.extractSensorIndex(filename)

	// Generate all sensor file paths
	paths := s.generateSensorPaths(hwmonDir, sensorIndex)

	// Read sensor label or use default
	label := s.readSensorLabel(paths.Label, sensorIndex)

	// Generate unique sensor ID
	hwmonID := filepath.Base(hwmonDir)
	sensorID := fmt.Sprintf("%s_%s", hwmonID, label)

	sensor := temperatureSensor{
		ID:         sensorID,
		Name:       deviceName,
		DeviceType: deviceType,
		Parent:     nil,
		Location:   label,
		InputPath:  paths.Input,
		LabelPath:  paths.Label,
		LimitPaths: map[metadata.AttributeLimitType]string{
			metadata.AttributeLimitTypeHighDegraded: paths.Max,
			metadata.AttributeLimitTypeHighCritical: paths.MaxCrit,
			metadata.AttributeLimitTypeLowDegraded:  paths.Min,
			metadata.AttributeLimitTypeLowCritical:  paths.MinCrit,
		},
	}

	return sensor, nil
}

// scrapeTemperatureSensor scrapes metrics from a single temperature sensor
func (s *temperatureScraper) scrapeTemperatureSensor(sensor temperatureSensor, timestamp time.Time) error {
	// Read current temperature
	temperature, err := s.readTemperature(sensor.InputPath)
	if err != nil {
		return fmt.Errorf("failed to read temperature: %w", err)
	}

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
	if s.config.Metrics.HwTemperatureLimit.Enabled {
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

// scrapeLimitMetrics scrapes temperature limit metrics for all available thresholds
func (s *temperatureScraper) scrapeLimitMetrics(sensor temperatureSensor, timestamp time.Time) {
	for limitType, limitPath := range sensor.LimitPaths {
		temp, err := s.readTemperature(limitPath)
		if err != nil {
			// Limit files are optional, so debug level is appropriate
			s.logger.Debug("Failed to read temperature limit",
				zap.String("sensor_id", sensor.ID),
				zap.String("limit_type", limitType.String()),
				zap.String("path", limitPath),
				zap.Error(err))
			continue
		}

		s.mb.RecordHwTemperatureLimitDataPoint(
			pcommon.NewTimestampFromTime(timestamp),
			temp,
			sensor.ID,
			limitType,
			sensor.Name,
			sensor.Parent,
			sensor.Location,
		)
	}
}

// Helper functions

// extractSensorIndex extracts sensor index from temp input filename
func (s *temperatureScraper) extractSensorIndex(filename string) string {
	return strings.TrimSuffix(strings.TrimPrefix(filename, "temp"), "_input")
}

// readSensorLabel reads sensor label from file or generates default
func (s *temperatureScraper) readSensorLabel(labelPath, sensorIndex string) string {
	if label, _ := s.readFile(labelPath); label != "" {
		return label
	}
	return fmt.Sprintf("temp%s", sensorIndex)
}

func (s *temperatureScraper) readDeviceName(hwmonDir string) (string, error) {
	nameFile := filepath.Join(hwmonDir, "name")
	if name, _ := s.readFile(nameFile); name != "" {
		return name, nil
	}
	return filepath.Base(hwmonDir), nil
}

func (s *temperatureScraper) shouldIncludeDevice(deviceName string, deviceType metadata.AttributeType) bool {
	// Log unknown device types for debugging
	if deviceType == metadata.AttributeTypeUnknown {
		s.logger.Debug("Unknown device type detected",
			zap.String("device_name", deviceName),
			zap.String("device_type", deviceType.String()))
	}

	if len(s.config.Devices.Types) == 0 {
		// Include unknown devices by default when no type filter is configured
		return true
	}

	deviceTypeStr := deviceType.String()
	for _, allowedType := range s.config.Devices.Types {
		if deviceTypeStr == allowedType {
			return true
		}
	}

	// Exclude if a device type doesn't match any allowed types
	s.logger.Debug("Device excluded by type filter",
		zap.String("device_name", deviceName),
		zap.String("device_type", deviceTypeStr),
		zap.Strings("allowed_types", s.config.Devices.Types))

	return false
}

// readFile reads a file and returns its trimmed content
func (s *temperatureScraper) readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// readTemperature reads and converts temperature from a sensor file
func (s *temperatureScraper) readTemperature(path string) (float64, error) {
	tempStr, err := s.readFile(path)
	if err != nil {
		return 0, err
	}

	tempMilliCelsius, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse temperature: %w", err)
	}

	return s.convertTemperature(tempMilliCelsius)
}

// detectDeviceTypeFromName attempts to determine a device type from name
func detectDeviceTypeFromName(name string) metadata.AttributeType {
	lowerName := strings.ToLower(name)

	// Check each device type pattern using the map
	for deviceType, patterns := range deviceTypePatterns {
		if containsAny(lowerName, patterns) {
			return deviceType
		}
	}

	// If no pattern matches, return unknown
	return metadata.AttributeTypeUnknown
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
