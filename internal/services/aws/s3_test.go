package aws

import (
	"context"
	"fmt"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type mockS3Client struct {
	listBucketsFunc           func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	listObjectsV2Func         func(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	headObjectFunc            func(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	getBucketLocationFunc     func(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	getBucketAclFunc          func(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error)
	getBucketPolicyStatusFunc func(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error)
	getBucketVersioningFunc   func(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error)
}

func (m *mockS3Client) ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
	return m.listBucketsFunc(ctx, params, optFns...)
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return m.listObjectsV2Func(ctx, params, optFns...)
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return m.headObjectFunc(ctx, params, optFns...)
}

func (m *mockS3Client) GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
	if m.getBucketLocationFunc != nil {
		return m.getBucketLocationFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketLocationOutput{}, nil
}

func (m *mockS3Client) GetBucketAcl(ctx context.Context, params *s3.GetBucketAclInput, optFns ...func(*s3.Options)) (*s3.GetBucketAclOutput, error) {
	if m.getBucketAclFunc != nil {
		return m.getBucketAclFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketAclOutput{}, nil
}

func (m *mockS3Client) GetBucketPolicyStatus(ctx context.Context, params *s3.GetBucketPolicyStatusInput, optFns ...func(*s3.Options)) (*s3.GetBucketPolicyStatusOutput, error) {
	if m.getBucketPolicyStatusFunc != nil {
		return m.getBucketPolicyStatusFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketPolicyStatusOutput{}, nil
}

func (m *mockS3Client) GetBucketVersioning(ctx context.Context, params *s3.GetBucketVersioningInput, optFns ...func(*s3.Options)) (*s3.GetBucketVersioningOutput, error) {
	if m.getBucketVersioningFunc != nil {
		return m.getBucketVersioningFunc(ctx, params, optFns...)
	}
	return &s3.GetBucketVersioningOutput{}, nil
}

func TestListBuckets_Success(t *testing.T) {
	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockS3Client{
		listBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: awssdk.String("bucket-b"), CreationDate: &created},
					{Name: awssdk.String("bucket-a"), CreationDate: &created},
				},
			}, nil
		},
		getBucketLocationFunc: func(_ context.Context, in *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
			switch awssdk.ToString(in.Bucket) {
			case "bucket-a":
				return &s3.GetBucketLocationOutput{LocationConstraint: s3types.BucketLocationConstraintApNortheast2}, nil
			case "bucket-b":
				return &s3.GetBucketLocationOutput{LocationConstraint: ""}, nil
			default:
				return nil, fmt.Errorf("unexpected bucket")
			}
		},
	}

	repo := &AwsRepository{S3Client: mock}
	buckets, err := repo.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(buckets))
	}
	if buckets[0].Name != "bucket-a" || buckets[0].Region != "ap-northeast-2" {
		t.Fatalf("unexpected first bucket: %+v", buckets[0])
	}
	if buckets[1].Region != "us-east-1" {
		t.Fatalf("expected us-east-1 fallback, got %s", buckets[1].Region)
	}
}

func TestListBuckets_UsesUnknownForNonFatalRegionErrors(t *testing.T) {
	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockS3Client{
		listBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: awssdk.String("bucket-a"), CreationDate: &created},
				},
			}, nil
		},
		getBucketLocationFunc: func(_ context.Context, _ *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "AccessDenied", Message: "access denied"}
		},
	}

	repo := &AwsRepository{S3Client: mock}
	buckets, err := repo.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].Region != "unknown" {
		t.Fatalf("expected unknown region for non-fatal error, got %q", buckets[0].Region)
	}
}

func TestListBuckets_FailsOnFatalRegionErrors(t *testing.T) {
	created := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	mock := &mockS3Client{
		listBucketsFunc: func(_ context.Context, _ *s3.ListBucketsInput, _ ...func(*s3.Options)) (*s3.ListBucketsOutput, error) {
			return &s3.ListBucketsOutput{
				Buckets: []s3types.Bucket{
					{Name: awssdk.String("bucket-a"), CreationDate: &created},
				},
			}, nil
		},
		getBucketLocationFunc: func(_ context.Context, _ *s3.GetBucketLocationInput, _ ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "InternalError", Message: "boom"}
		},
	}

	repo := &AwsRepository{S3Client: mock}
	_, err := repo.ListBuckets(context.Background())
	if err == nil {
		t.Fatal("expected fatal region error, got nil")
	}
}

func TestListBucketObjects_Success(t *testing.T) {
	modified := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mock := &mockS3Client{
		listObjectsV2Func: func(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
			if awssdk.ToString(in.Bucket) != "my-bucket" {
				t.Fatalf("unexpected bucket %q", awssdk.ToString(in.Bucket))
			}
			if awssdk.ToString(in.Prefix) != "logs/" {
				t.Fatalf("unexpected prefix %q", awssdk.ToString(in.Prefix))
			}
			if awssdk.ToString(in.Delimiter) != "/" {
				t.Fatalf("expected delimiter /")
			}
			return &s3.ListObjectsV2Output{
				CommonPrefixes: []s3types.CommonPrefix{
					{Prefix: awssdk.String("logs/app1/")},
				},
				Contents: []s3types.Object{
					{Key: awssdk.String("logs/file.txt"), Size: awssdk.Int64(42), LastModified: &modified, StorageClass: s3types.ObjectStorageClassStandard},
					{Key: awssdk.String("logs/app1/nested.txt"), Size: awssdk.Int64(99), LastModified: &modified},
				},
			}, nil
		},
	}

	repo := &AwsRepository{S3Client: mock}
	result, err := repo.ListBucketObjects(context.Background(), "my-bucket", "logs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Prefixes) != 1 || result.Prefixes[0].Name != "app1" {
		t.Fatalf("unexpected prefixes: %+v", result.Prefixes)
	}
	if len(result.Objects) != 1 || result.Objects[0].Name != "file.txt" {
		t.Fatalf("unexpected objects: %+v", result.Objects)
	}
}

func TestHeadBucketObject_Success(t *testing.T) {
	modified := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	mock := &mockS3Client{
		headObjectFunc: func(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
			if awssdk.ToString(in.Bucket) != "my-bucket" || awssdk.ToString(in.Key) != "logs/file.txt" {
				t.Fatalf("unexpected head input: bucket=%q key=%q", awssdk.ToString(in.Bucket), awssdk.ToString(in.Key))
			}
			return &s3.HeadObjectOutput{
				ContentLength: awssdk.Int64(42),
				LastModified:  &modified,
				StorageClass:  s3types.StorageClassStandard,
				ContentType:   awssdk.String("text/plain"),
				ETag:          awssdk.String(`"etag123"`),
			}, nil
		},
	}

	repo := &AwsRepository{S3Client: mock}
	detail, err := repo.HeadBucketObject(context.Background(), "my-bucket", "logs/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if detail.ContentType != "text/plain" || detail.ETag != "etag123" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestS3PrefixHelpers(t *testing.T) {
	if got := NormalizeS3Prefix("logs/app"); got != "logs/app/" {
		t.Fatalf("expected normalized prefix, got %q", got)
	}
	if got := ParentS3Prefix("logs/app/"); got != "logs/" {
		t.Fatalf("expected parent prefix logs/, got %q", got)
	}
	if got := S3Breadcrumb("logs/app/"); got != "/logs/app" {
		t.Fatalf("unexpected breadcrumb %q", got)
	}
}
