package fetcher

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &serverBackup{}

// NewServerBackup creates a new fetcher that will collect pricing information on server backups.
func NewServerBackup(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &serverBackup{newBase(pricing, "server_backup", []string{"location", "type"}, additionalLabels...)}
}

type serverBackup struct {
	*baseFetcher
}

func (serverBackup serverBackup) Run(client *hcloud.Client) error {
	servers, err := getServer(client) // Use existing helper
	if err != nil {
		return fmt.Errorf("failed to list servers for backup pricing: %w", err)
	}

	backupPercentage, err := serverBackup.pricing.ServerBackup() // Get price once
	if err != nil {
		return fmt.Errorf("could not get server backup pricing: %w", err)
	}

	for _, s := range servers {
		location := s.Datacenter.Location

		labels := append([]string{
			s.Name,
			location.Name,
			s.ServerType.Name,
		},
			parseAdditionalLabels(serverBackup.additionalLabels, s.Labels)...,
		)

		if s.BackupWindow != "" {
			serverPriceInfo, err := findServerPricing(location, s.ServerType.Pricings)
			if err != nil {
				// Log or return error? Return seems consistent.
				log.Printf("Could not find server pricing for %s (%s) needed for backup calculation: %v", s.Name, location.Name, err)
				return fmt.Errorf("could not find server pricing for %s (%s) needed for backup calculation: %w", s.Name, location.Name, err)
			}

			// Use the adjusted helper function
			hourlyPrice := calculateBackupPrice(serverPriceInfo.Hourly.Gross, backupPercentage)
			monthlyPrice := calculateBackupPrice(serverPriceInfo.Monthly.Gross, backupPercentage)

			serverBackup.hourly.WithLabelValues(labels...).Set(hourlyPrice)
			serverBackup.monthly.WithLabelValues(labels...).Set(monthlyPrice)
		} else {
			serverBackup.hourly.WithLabelValues(labels...).Set(0)
			serverBackup.monthly.WithLabelValues(labels...).Set(0)
		}
	}

	return nil
}

// calculateBackupPrice calculates the backup price based on server price and backup percentage.
func calculateBackupPrice(rawServerPrice string, backupPercentage float64) float64 {
	serverPrice, err := strconv.ParseFloat(rawServerPrice, 64) // Use 64 bit
	if err != nil {
		log.Printf("Error parsing server price '%s' for backup calculation: %v", rawServerPrice, err)
		return 0
	}
	if backupPercentage <= 0 {
		return 0
	}
	return serverPrice * (backupPercentage / 100)
}
