package aws

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	uniclog "unic/internal/log"
)

func (r *AwsRepository) ListBuckets(ctx context.Context) ([]S3Bucket, error) {
	uniclog.Debug("aws", "ListBuckets called")
	out, err := r.S3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 buckets: %w", err)
	}

	buckets := make([]S3Bucket, 0, len(out.Buckets))
	for _, bucket := range out.Buckets {
		name := awssdk.ToString(bucket.Name)
		region, err := r.bucketRegion(ctx, name)
		if err != nil {
			return nil, err
		}
		item := S3Bucket{
			Name:   name,
			Region: region,
		}
		if bucket.CreationDate != nil {
			item.CreationDate = *bucket.CreationDate
		}
		buckets = append(buckets, item)
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})
	return buckets, nil
}

func (r *AwsRepository) ListBucketObjects(ctx context.Context, bucketName, prefix string) (S3ListResult, error) {
	uniclog.Debug("aws", "ListBucketObjects called", "bucket", bucketName, "prefix", prefix)

	prefix = NormalizeS3Prefix(prefix)
	out, err := r.S3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    awssdk.String(bucketName),
		Prefix:    awssdk.String(prefix),
		Delimiter: awssdk.String("/"),
	})
	if err != nil {
		return S3ListResult{}, fmt.Errorf("failed to list objects for s3://%s/%s: %w", bucketName, prefix, err)
	}

	var result S3ListResult
	for _, commonPrefix := range out.CommonPrefixes {
		fullPrefix := NormalizeS3Prefix(awssdk.ToString(commonPrefix.Prefix))
		name := strings.TrimSuffix(strings.TrimPrefix(fullPrefix, prefix), "/")
		if name == "" {
			continue
		}
		result.Prefixes = append(result.Prefixes, S3Object{
			Key:      fullPrefix,
			Name:     name,
			Prefix:   fullPrefix,
			IsPrefix: true,
		})
	}

	for _, object := range out.Contents {
		key := awssdk.ToString(object.Key)
		if key == prefix {
			continue
		}
		relative := strings.TrimPrefix(key, prefix)
		if strings.Contains(relative, "/") || relative == "" {
			continue
		}
		item := S3Object{
			Key:          key,
			Name:         path.Base(key),
			Size:         awssdk.ToInt64(object.Size),
			StorageClass: string(object.StorageClass),
		}
		if object.LastModified != nil {
			item.LastModified = *object.LastModified
		}
		result.Objects = append(result.Objects, item)
	}

	sort.Slice(result.Prefixes, func(i, j int) bool {
		return result.Prefixes[i].Name < result.Prefixes[j].Name
	})
	sort.Slice(result.Objects, func(i, j int) bool {
		return result.Objects[i].Name < result.Objects[j].Name
	})

	return result, nil
}

func (r *AwsRepository) HeadBucketObject(ctx context.Context, bucketName, key string) (*S3ObjectDetail, error) {
	uniclog.Debug("aws", "HeadBucketObject called", "bucket", bucketName, "key", key)

	out, err := r.S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: awssdk.String(bucketName),
		Key:    awssdk.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to head object s3://%s/%s: %w", bucketName, key, err)
	}

	detail := &S3ObjectDetail{
		Bucket:       bucketName,
		Key:          key,
		Size:         awssdk.ToInt64(out.ContentLength),
		StorageClass: string(out.StorageClass),
		ContentType:  awssdk.ToString(out.ContentType),
		ETag:         strings.Trim(awssdk.ToString(out.ETag), `"`),
	}
	if out.LastModified != nil {
		detail.LastModified = *out.LastModified
	}
	if detail.StorageClass == "" {
		detail.StorageClass = string(s3types.ObjectStorageClassStandard)
	}
	if detail.ContentType == "" {
		detail.ContentType = "-"
	}
	return detail, nil
}

func (r *AwsRepository) bucketRegion(ctx context.Context, bucketName string) (string, error) {
	out, err := r.S3Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: awssdk.String(bucketName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get region for bucket %s: %w", bucketName, err)
	}

	switch out.LocationConstraint {
	case "":
		return "us-east-1", nil
	case s3types.BucketLocationConstraintEu:
		return "eu-west-1", nil
	default:
		return string(out.LocationConstraint), nil
	}
}
