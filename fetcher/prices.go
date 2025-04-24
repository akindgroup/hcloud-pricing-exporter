package fetcher

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/hetznercloud/hcloud-go/hcloud"
)

// PriceProvider provides easy access to current HCloud prices.
type PriceProvider struct {
	Client      *hcloud.Client
	pricing     *hcloud.Pricing
	pricingLock sync.RWMutex
}

// getPricing fetches pricing information if not already cached.
// It handles locking internally and returns an error if fetching fails.
func (provider *PriceProvider) getPricing() (*hcloud.Pricing, error) {
	provider.pricingLock.RLock()
	// Fast path: check if pricing is already cached
	if provider.pricing != nil {
		p := provider.pricing
		provider.pricingLock.RUnlock()
		return p, nil
	}
	// Release read lock before acquiring write lock
	provider.pricingLock.RUnlock()

	provider.pricingLock.Lock()
	defer provider.pricingLock.Unlock()
	// Double-check after acquiring write lock, another goroutine might have fetched it.
	if provider.pricing != nil {
		return provider.pricing, nil
	}

	// Fetch pricing
	log.Println("Pricing cache empty or invalidated, fetching from HCloud API...")
	pricing, _, err := provider.Client.Pricing.Get(context.Background())
	if err != nil {
		log.Printf("Error fetching pricing from HCloud API: %v", err)
		return nil, fmt.Errorf("failed to fetch pricing from API: %w", err)
	}

	log.Println("Successfully fetched pricing information from API.")
	provider.pricing = &pricing
	return provider.pricing, nil
}

// FloatingIP returns the current price for a floating IP per month.
func (provider *PriceProvider) FloatingIP(ipType hcloud.FloatingIPType, location string) (float64, error) {
	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, fmt.Errorf("failed to get pricing information: %w", err)
	}

	for _, byType := range pricingInfo.FloatingIPs {
		if byType.Type == ipType {
			for _, pricing := range byType.Pricings {
				if pricing.Location.Name == location {
					return parsePrice(pricing.Monthly.Gross), nil
				}
			}
		}
	}

	// Fallback logic removed, assume API provides specific pricing or it's an error.
	return 0, fmt.Errorf("no floating IP pricing found for type %s in location %s", ipType, location)
}

// PrimaryIP returns the current price for a primary IP per hour and month.
func (provider *PriceProvider) PrimaryIP(ipType hcloud.PrimaryIPType, location string) (hourly, monthly float64, err error) {
	// v6 pricing is not defined by the API
	if string(ipType) == "ipv6" {
		return 0, 0, nil
	}

	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get pricing information: %w", err)
	}

	for _, byType := range pricingInfo.PrimaryIPs {
		if byType.Type == string(ipType) {
			for _, pricing := range byType.Pricings {
				// API uses Location.Name for Primary IPs pricing location identifier
				if pricing.Location == location {
					return parsePrice(pricing.Hourly.Gross), parsePrice(pricing.Monthly.Gross), nil
				}
			}
		}
	}

	return 0, 0, fmt.Errorf("no primary IP pricing found for type %s in location %s", ipType, location)
}

// Image returns the current price for an image per GB per month.
func (provider *PriceProvider) Image() (float64, error) {
	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, fmt.Errorf("failed to get pricing information: %w", err)
	}
	return parsePrice(pricingInfo.Image.PerGBMonth.Gross), nil
}

// Traffic returns the current price for a TB of extra traffic per month.
func (provider *PriceProvider) Traffic() (float64, error) {
	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, fmt.Errorf("failed to get pricing information: %w", err)
	}
	return parsePrice(pricingInfo.Traffic.PerTB.Gross), nil
}

// ServerBackup returns the percentage of base price increase for server backups per month.
func (provider *PriceProvider) ServerBackup() (float64, error) {
	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, fmt.Errorf("failed to get pricing information: %w", err)
	}
	return parsePrice(pricingInfo.ServerBackup.Percentage), nil
}

// Volume returns the current price for a volume per GB per month.
func (provider *PriceProvider) Volume() (float64, error) {
	pricingInfo, err := provider.getPricing()
	if err != nil {
		return 0, fmt.Errorf("failed to get pricing information: %w", err)
	}
	return parsePrice(pricingInfo.Volume.PerGBMonthly.Gross), nil
}

// Sync forces the provider to re-fetch prices on the next access.
func (provider *PriceProvider) Sync() {
	provider.pricingLock.Lock()         // Acquire Write lock
	defer provider.pricingLock.Unlock() // Release Write lock

	log.Println("Invalidating pricing cache.")
	provider.pricing = nil // Clear the cache
}

func parsePrice(rawPrice string) float64 {
	if price, err := strconv.ParseFloat(rawPrice, 32); err == nil {
		return price
	}

	return 0
}
