package aws

import (
	"context"
	"sync"
)

// RegionError reports a per-region listing failure during an all-regions scan.
type RegionError struct {
	Region string
	Err    error
}

// repoForRegionFn is a test seam: ForRegion builds real SDK clients, so tests
// substitute regional repositories with mocked clients here.
var repoForRegionFn = func(r *AwsRepository, region string) *AwsRepository {
	return r.ForRegion(region)
}

// listAcrossRegions fans a per-region list call out concurrently, reusing the
// repository credentials via ForRegion. Per-region failures are returned
// alongside the successful rows instead of failing the whole scan.
func listAcrossRegions[T any](ctx context.Context, r *AwsRepository, regions []string, list func(context.Context, *AwsRepository) ([]T, error)) ([]T, []RegionError) {
	type regionResult struct {
		items []T
		err   error
	}
	results := make([]regionResult, len(regions))
	var wg sync.WaitGroup
	for i, region := range regions {
		wg.Add(1)
		go func(i int, region string) {
			defer wg.Done()
			repo := r
			if region != r.Region {
				repo = repoForRegionFn(r, region)
			}
			items, err := list(ctx, repo)
			results[i] = regionResult{items: items, err: err}
		}(i, region)
	}
	wg.Wait()

	var items []T
	var regionErrors []RegionError
	for i, result := range results {
		if result.err != nil {
			regionErrors = append(regionErrors, RegionError{Region: regions[i], Err: result.err})
			continue
		}
		items = append(items, result.items...)
	}
	return items, regionErrors
}
