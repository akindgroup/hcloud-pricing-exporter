package fetcher

import (
	"fmt"
	"log"
	"math"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &loadbalancerTraffic{}

// NewLoadbalancerTraffic creates a new fetcher that will collect pricing information on load balancer traffic.
func NewLoadbalancerTraffic(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &loadbalancerTraffic{newBase(pricing, "loadbalancer_traffic", []string{"location", "type"}, additionalLabels...)}
}

type loadbalancerTraffic struct {
	*baseFetcher
}

func (loadbalancerTraffic loadbalancerTraffic) Run(client *hcloud.Client) error {
	loadBalancers, _, err := client.LoadBalancer.List(ctx, hcloud.LoadBalancerListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list load balancers for traffic pricing: %w", err)
	}

	trafficPricePerTB, err := loadbalancerTraffic.pricing.Traffic()
	if err != nil {
		log.Printf("Could not get traffic pricing: %v", err)
		return fmt.Errorf("could not get traffic pricing: %w", err)
	}

	for _, lb := range loadBalancers {
		location := lb.Location

		labels := append([]string{
			lb.Name,
			location.Name,
			lb.LoadBalancerType.Name,
		},
			parseAdditionalLabels(loadbalancerTraffic.additionalLabels, lb.Labels)...,
		)

		additionalTraffic := int(lb.OutgoingTraffic) - int(lb.IncludedTraffic)
		if additionalTraffic < 0 {
			loadbalancerTraffic.hourly.WithLabelValues(labels...).Set(0)
			loadbalancerTraffic.monthly.WithLabelValues(labels...).Set(0)
			continue // Use continue instead of break to process other load balancers
		}

		monthlyPrice := math.Ceil(float64(additionalTraffic)/sizeTB) * trafficPricePerTB
		hourlyPrice := pricingPerHour(monthlyPrice)

		loadbalancerTraffic.hourly.WithLabelValues(labels...).Set(hourlyPrice)
		loadbalancerTraffic.monthly.WithLabelValues(labels...).Set(monthlyPrice)
	}

	return nil
}
