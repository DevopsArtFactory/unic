package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

type mockECRClient struct {
	describeRepositoriesFunc func(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
	describeImagesFunc       func(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
}

func (m *mockECRClient) DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
	return m.describeRepositoriesFunc(ctx, params, optFns...)
}

func (m *mockECRClient) DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
	if m.describeImagesFunc != nil {
		return m.describeImagesFunc(ctx, params, optFns...)
	}
	return &ecr.DescribeImagesOutput{}, nil
}

func TestBuildPrivateECRRegistryURI(t *testing.T) {
	got, err := BuildPrivateECRRegistryURI("123456789012", "ap-northeast-2")
	if err != nil {
		t.Fatal(err)
	}
	want := "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildPrivateECRRegistryURIRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		accountID string
		region    string
	}{
		{name: "invalid account", accountID: "1234abc", region: "ap-northeast-2"},
		{name: "invalid region", accountID: "123456789012", region: "ap-northeast-2; rm -rf /"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildPrivateECRRegistryURI(tc.accountID, tc.region); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBuildECRLoginCommandDocker(t *testing.T) {
	got, err := BuildECRLoginCommand("123456789012.dkr.ecr.ap-northeast-2.amazonaws.com", "ap-northeast-2", ECRRuntimeDocker)
	if err != nil {
		t.Fatal(err)
	}
	want := "aws ecr get-login-password --region ap-northeast-2 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.ap-northeast-2.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildECRLoginCommandPodman(t *testing.T) {
	got, err := BuildECRLoginCommand("123456789012.dkr.ecr.us-east-1.amazonaws.com", "us-east-1", ECRRuntimePodman)
	if err != nil {
		t.Fatal(err)
	}
	want := "aws ecr get-login-password --region us-east-1 | podman login --username AWS --password-stdin 123456789012.dkr.ecr.us-east-1.amazonaws.com"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildECRLoginCommandRejectsInjectionAttempts(t *testing.T) {
	tests := []struct {
		name        string
		registryURI string
		region      string
	}{
		{
			name:        "region injection",
			registryURI: "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com",
			region:      "ap-northeast-2; rm -rf /",
		},
		{
			name:        "registry injection",
			registryURI: "123456789012.dkr.ecr.ap-northeast-2.amazonaws.com; curl bad",
			region:      "ap-northeast-2",
		},
		{
			name:        "registry region mismatch",
			registryURI: "123456789012.dkr.ecr.us-east-1.amazonaws.com",
			region:      "ap-northeast-2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildECRLoginCommand(tc.registryURI, tc.region, ECRRuntimeDocker)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if strings.Contains(err.Error(), "; rm -rf /") || strings.Contains(err.Error(), "curl bad") {
				t.Fatalf("error should not echo malicious shell payload verbatim: %v", err)
			}
		})
	}
}

func TestParseECRRuntimeRejectsUnknown(t *testing.T) {
	if _, err := ParseECRRuntime("nerdctl"); err == nil {
		t.Fatal("expected unsupported runtime error")
	}
}

func TestListECRRepositoriesSuccess(t *testing.T) {
	mock := &mockECRClient{
		describeRepositoriesFunc: func(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{
				Repositories: []ecrtypes.Repository{
					{
						RepositoryName: awssdk.String("zeta"),
						RepositoryUri:  awssdk.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/zeta"),
						RegistryId:     awssdk.String("123456789012"),
					},
					{
						RepositoryName:     awssdk.String("app"),
						RepositoryUri:      awssdk.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/app"),
						RepositoryArn:      awssdk.String("arn:aws:ecr:us-east-1:123456789012:repository/app"),
						RegistryId:         awssdk.String("123456789012"),
						ImageTagMutability: ecrtypes.ImageTagMutabilityImmutable,
						ImageScanningConfiguration: &ecrtypes.ImageScanningConfiguration{
							ScanOnPush: true,
						},
						EncryptionConfiguration: &ecrtypes.EncryptionConfiguration{
							EncryptionType: ecrtypes.EncryptionTypeKms,
							KmsKey:         awssdk.String("arn:aws:kms:us-east-1:123456789012:key/key-id"),
						},
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{ECRClient: mock}
	repositories, err := repo.ListECRRepositories(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repositories) != 2 {
		t.Fatalf("expected 2 repositories, got %d", len(repositories))
	}

	got := repositories[0]
	if got.Name != "app" {
		t.Fatalf("expected sorted repository app first, got %q", got.Name)
	}
	if got.URI != "123456789012.dkr.ecr.us-east-1.amazonaws.com/app" {
		t.Errorf("unexpected URI: %q", got.URI)
	}
	if !got.ScanOnPush {
		t.Error("expected scan-on-push to be true")
	}
	if got.TagMutability != "IMMUTABLE" {
		t.Errorf("expected IMMUTABLE mutability, got %q", got.TagMutability)
	}
	if got.Encryption != "KMS (key-id)" {
		t.Errorf("expected shortened KMS encryption summary, got %q", got.Encryption)
	}
	if repositories[1].Encryption != "AES256" {
		t.Errorf("expected default AES256 encryption summary, got %q", repositories[1].Encryption)
	}
}

func TestListECRRepositoriesError(t *testing.T) {
	mock := &mockECRClient{
		describeRepositoriesFunc: func(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{ECRClient: mock}
	_, err := repo.ListECRRepositories(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestECRRepositoryFilterText(t *testing.T) {
	repo := ECRRepository{
		Name:          "App",
		URI:           "123456789012.dkr.ecr.us-east-1.amazonaws.com/app",
		RegistryID:    "123456789012",
		ARN:           "arn:aws:ecr:us-east-1:123456789012:repository/app",
		TagMutability: "IMMUTABLE",
		Encryption:    "KMS (alias/ecr)",
	}

	filterText := repo.FilterText()
	for _, want := range []string{"app", "123456789012", "immutable", "kms"} {
		if !strings.Contains(filterText, want) {
			t.Errorf("FilterText %q should contain %q", filterText, want)
		}
	}
}

func TestListECRImagesSuccess(t *testing.T) {
	pushedRecent := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	pushedOld := time.Date(2025, 12, 1, 10, 0, 0, 0, time.UTC)
	mock := &mockECRClient{
		describeRepositoriesFunc: func(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{}, nil
		},
		describeImagesFunc: func(_ context.Context, params *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
			if awssdk.ToString(params.RepositoryName) != "app" {
				t.Fatalf("expected repository app, got %q", awssdk.ToString(params.RepositoryName))
			}
			return &ecr.DescribeImagesOutput{
				ImageDetails: []ecrtypes.ImageDetail{
					{
						ImageDigest:      awssdk.String("sha256:old"),
						ImageTags:        []string{"old"},
						ImagePushedAt:    awssdk.Time(pushedOld),
						ImageSizeInBytes: awssdk.Int64(2048),
					},
					{
						ImageDigest:      awssdk.String("sha256:recent"),
						ImageTags:        []string{"latest", "v1"},
						ImagePushedAt:    awssdk.Time(pushedRecent),
						ImageSizeInBytes: awssdk.Int64(1024),
					},
					{
						ImageDigest:      awssdk.String("sha256:untagged"),
						ImagePushedAt:    awssdk.Time(pushedRecent.Add(-time.Hour)),
						ImageSizeInBytes: awssdk.Int64(512),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{ECRClient: mock}
	images, err := repo.ListECRImages(context.Background(), "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}
	if images[0].Digest != "sha256:recent" {
		t.Fatalf("expected most recent image first, got %q", images[0].Digest)
	}
	if images[0].TagsText() != "latest, v1" {
		t.Errorf("unexpected tag text: %q", images[0].TagsText())
	}
	if images[1].TagsText() != "(untagged)" {
		t.Errorf("expected untagged marker, got %q", images[1].TagsText())
	}
	if !images[1].IsUntagged() {
		t.Error("expected untagged image")
	}
	if !images[2].IsStale(time.Date(2026, 4, 26, 0, 0, 0, 0, time.UTC)) {
		t.Error("expected old image to be stale")
	}
}

func TestListECRImagesError(t *testing.T) {
	mock := &mockECRClient{
		describeRepositoriesFunc: func(_ context.Context, _ *ecr.DescribeRepositoriesInput, _ ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error) {
			return &ecr.DescribeRepositoriesOutput{}, nil
		},
		describeImagesFunc: func(_ context.Context, _ *ecr.DescribeImagesInput, _ ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{ECRClient: mock}
	_, err := repo.ListECRImages(context.Background(), "app")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
