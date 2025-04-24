package fetcher

import (
	"fmt"
	"log"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &floatingIP{}

// NewFloatingIP creates a new fetcher that will collect pricing information on floating IPs.
func NewFloatingIP(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &floatingIP{newBase(pricing, "floatingip", []string{"location", "type"}, additionalLabels...)}
}

type floatingIP struct {
	*baseFetcher
}

func (floatingIP floatingIP) Run(client *hcloud.Client) error {
	floatingIPs, _, err := client.FloatingIP.List(ctx, hcloud.FloatingIPListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list floating IPs: %w", err)
	}

	for _, f := range floatingIPs {
		location := f.HomeLocation

		monthlyPrice, err := floatingIP.pricing.FloatingIP(f.Type, location.Name)
		if err != nil {
			log.Printf("Could not get floating IP pricing for %s (%s, %s): %v", f.Name, f.Type, location.Name, err)
			return fmt.Errorf("could not get floating IP pricing for %s (%s, %s): %w", f.Name, f.Type, location.Name, err)
		}
		hourlyPrice := pricingPerHour(monthlyPrice)

		labels := append([]string{
			f.Name,
			location.Name,
			string(f.Type),
		},
			parseAdditionalLabels(floatingIP.additionalLabels, f.Labels)...,
		)

		floatingIP.hourly.WithLabelValues(labels...).Set(hourlyPrice)
		floatingIP.monthly.WithLabelValues(labels...).Set(monthlyPrice)
	}

	return nil
}
