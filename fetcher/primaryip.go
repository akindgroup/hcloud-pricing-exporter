package fetcher

import (
	"fmt"
	"log"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &floatingIP{}

// NewPrimaryIP creates a new fetcher that will collect pricing information on primary IPs.
func NewPrimaryIP(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &primaryIP{newBase(pricing, "primaryip", []string{"datacenter", "type"}, additionalLabels...)}
}

type primaryIP struct {
	*baseFetcher
}

func (primaryIP primaryIP) Run(client *hcloud.Client) error {
	primaryIPs, _, err := client.PrimaryIP.List(ctx, hcloud.PrimaryIPListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list primary IPs: %w", err) // Wrap error
	}

	for _, p := range primaryIPs {
		datacenter := p.Datacenter

		// Get pricing, handle potential error
		hourlyPrice, monthlyPrice, err := primaryIP.pricing.PrimaryIP(p.Type, datacenter.Location.Name)
		if err != nil {
			// Log the error and return it to stop this fetcher's run and report the issue.
			log.Printf("Could not get primary IP pricing for %s (%s, %s): %v", p.Name, p.Type, datacenter.Location.Name, err)
			return fmt.Errorf("could not get primary IP pricing for %s (%s, %s): %w", p.Name, p.Type, datacenter.Location.Name, err)
		}

		labels := append([]string{
			p.Name,
			datacenter.Name,
			string(p.Type),
		},
			parseAdditionalLabels(primaryIP.additionalLabels, p.Labels)...,
		)

		primaryIP.hourly.WithLabelValues(labels...).Set(hourlyPrice)
		primaryIP.monthly.WithLabelValues(labels...).Set(monthlyPrice)
	}

	return nil
}
