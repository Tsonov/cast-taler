package server

import (
	"fmt"
	"math/rand"
	"os"

	"gopkg.in/yaml.v3"
)

type ZoneConfig map[string]Zone

type Zone struct {
	R200 int `yaml:"200"`
	R404 int `yaml:"404"`
	R500 int `yaml:"500"`
}

func LoadZoneConfig(path string) (*ZoneConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ZoneConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (z ZoneConfig) CheckResponsePercentage() error {
	for name, zone := range z {
		total := zone.R200 + zone.R404 + zone.R500
		if zone.R200 < 0 || zone.R404 < 0 || zone.R500 < 0 {
			return fmt.Errorf("response counts cannot be negative for zone %s", name)
		}
		if total != 100 {
			return fmt.Errorf("response percentages must sum to 100 for zone %s, got %d", name, total)
		}
	}
	return nil
}

func (z ZoneConfig) GetRandomCode(zoneName string) (int, error) {
	zone, ok := z[zoneName]
	if !ok {
		return 0, fmt.Errorf("zone %s not found", zoneName)
	}
	total := zone.R200 + zone.R404 + zone.R500
	if total == 0 {
		return 0, fmt.Errorf("no responses defined for zone %s", zoneName)
	}
	randNum := rand.Intn(total)
	if randNum < zone.R200 {
		return 200, fmt.Errorf("200 OK response for zone %s", zoneName)
	} else if randNum < zone.R200+zone.R404 {
		return 404, fmt.Errorf("404 Not Found response for zone %s", zoneName)
	} else {
		return 500, fmt.Errorf("500 Internal Server Error response for zone %s", zoneName)
	}
}
