package fetcher

import (
	"fmt"
	"log"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &snapshot{}

// NewSnapshot creates a new fetcher that will collect pricing information on server snapshots.
func NewSnapshot(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &snapshot{newBase(pricing, "snapshot", nil, additionalLabels...)}
}

type snapshot struct {
	*baseFetcher
}

func (snapshot snapshot) Run(client *hcloud.Client) error {
	images, _, err := client.Image.List(ctx, hcloud.ImageListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list images for snapshot pricing: %w", err)
	}

	snapshotPricePerGB, err := snapshot.pricing.Image()
	if err != nil {
		log.Printf("Could not get snapshot/image pricing: %v", err)
		return fmt.Errorf("could not get snapshot/image pricing: %w", err)
	}

	for _, i := range images {
		if i.Type == "snapshot" {
			monthlyPrice := float64(i.ImageSize) * snapshotPricePerGB
			hourlyPrice := pricingPerHour(monthlyPrice)

			labels := append([]string{
				i.Name,
			},
				parseAdditionalLabels(snapshot.additionalLabels, i.Labels)...,
			)

			snapshot.hourly.WithLabelValues(labels...).Set(hourlyPrice)
			snapshot.monthly.WithLabelValues(labels...).Set(monthlyPrice)
		}
	}

	return nil
}
