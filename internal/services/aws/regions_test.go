package aws

import (
	"context"
	"errors"
	"testing"
)

func TestListAcrossRegionsMergesAndReportsFailures(t *testing.T) {
	base := &AwsRepository{Region: "us-east-1"}
	west := &AwsRepository{Region: "eu-west-1"}

	orig := repoForRegionFn
	t.Cleanup(func() { repoForRegionFn = orig })
	repoForRegionFn = func(_ *AwsRepository, region string) *AwsRepository {
		if region != "eu-west-1" {
			t.Fatalf("unexpected region %s", region)
		}
		return west
	}

	wantErr := errors.New("region down")
	items, regionErrors := listAcrossRegions(context.Background(), base,
		[]string{"us-east-1", "eu-west-1"},
		func(_ context.Context, repo *AwsRepository) ([]string, error) {
			if repo.Region == "eu-west-1" {
				return nil, wantErr
			}
			return []string{"a-" + repo.Region, "b-" + repo.Region}, nil
		})

	if len(items) != 2 || items[0] != "a-us-east-1" {
		t.Fatalf("expected the healthy region's items, got %+v", items)
	}
	if len(regionErrors) != 1 || regionErrors[0].Region != "eu-west-1" || !errors.Is(regionErrors[0].Err, wantErr) {
		t.Fatalf("expected one eu-west-1 failure, got %+v", regionErrors)
	}
}
