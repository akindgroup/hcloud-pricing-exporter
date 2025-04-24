package fetcher

import (
	"fmt"
	"log"
	"strconv"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

var _ Fetcher = &volume{}

// NewVolume creates a new fetcher that will collect pricing information on volumes.
func NewVolume(pricing *PriceProvider, additionalLabels ...string) Fetcher {
	return &volume{newBase(pricing, "volume", []string{"location", "bytes"}, additionalLabels...)}
}

type volume struct {
	*baseFetcher
}

func (volume volume) Run(client *hcloud.Client) error {
	volumes, _, err := client.Volume.List(ctx, hcloud.VolumeListOpts{})
	if err != nil {
		return fmt.Errorf("failed to list volumes: %w", err)
	}

	volumePricePerGB, err := volume.pricing.Volume()
	if err != nil {
		log.Printf("Could not get volume pricing: %v", err)
		return fmt.Errorf("could not get volume pricing: %w", err)
	}

	for _, v := range volumes {
		monthlyPrice := float64(v.Size) * volumePricePerGB
		hourlyPrice := pricingPerHour(monthlyPrice)

		labels := append([]string{
			v.Name,
			v.Location.Name,
			strconv.Itoa(v.Size),
		},
			parseAdditionalLabels(volume.additionalLabels, v.Labels)...,
		)

		volume.hourly.WithLabelValues(labels...).Set(hourlyPrice)
		volume.monthly.WithLabelValues(labels...).Set(monthlyPrice)
	}

	return nil
}
