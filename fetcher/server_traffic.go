package fetcher

import (
	"fmt"
	"log"
	"math"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &serverTraffic{}

// NewServerTraffic creates a new fetcher that will collect pricing information on server traffic.
func NewServerTraffic(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &serverTraffic{newBase(pricing, "server_traffic", []string{"location", "type"}, additionalLabels...)}
}

type serverTraffic struct {
	*baseFetcher
}

func (serverTraffic serverTraffic) Run(client *hcloud.Client) error {
	servers, err := getServer(client) // Use existing helper
	if err != nil {
		return fmt.Errorf("failed to list servers for traffic pricing: %w", err)
	}

	trafficPricePerTB, err := serverTraffic.pricing.Traffic()
	if err != nil {
		log.Printf("Could not get traffic pricing: %v", err)
		return fmt.Errorf("could not get traffic pricing: %w", err)
	}

	for _, s := range servers {
		location := s.Datacenter.Location

		labels := append([]string{
			s.Name,
			location.Name,
			s.ServerType.Name,
		},
			parseAdditionalLabels(serverTraffic.additionalLabels, s.Labels)...,
		)

		additionalTraffic := int(s.OutgoingTraffic) - int(s.IncludedTraffic)
		if additionalTraffic < 0 {
			serverTraffic.hourly.WithLabelValues(labels...).Set(0)
			serverTraffic.monthly.WithLabelValues(labels...).Set(0)
			continue // Use continue instead of break to process other servers
		}

		monthlyPrice := math.Ceil(float64(additionalTraffic)/sizeTB) * trafficPricePerTB
		hourlyPrice := pricingPerHour(monthlyPrice)

		serverTraffic.hourly.WithLabelValues(labels...).Set(hourlyPrice)
		serverTraffic.monthly.WithLabelValues(labels...).Set(monthlyPrice)
	}

	return nil
}
