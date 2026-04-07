package aws

import (
	"fmt"
	"path"
	"strings"
	"time"
)

type S3Bucket struct {
	Name         string
	Region       string
	CreationDate time.Time
}

func (b S3Bucket) DisplayTitle() string {
	created := "-"
	if !b.CreationDate.IsZero() {
		created = b.CreationDate.Format(time.DateOnly)
	}
	return fmt.Sprintf("%s  [%s]  created:%s", b.Name, b.Region, created)
}

func (b S3Bucket) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s", b.Name, b.Region))
}

type S3Object struct {
	Key          string
	Name         string
	Prefix       string
	IsPrefix     bool
	Size         int64
	LastModified time.Time
	StorageClass string
}

func (o S3Object) DisplayTitle() string {
	if o.IsPrefix {
		return fmt.Sprintf("%s/", o.Name)
	}
	modified := "-"
	if !o.LastModified.IsZero() {
		modified = o.LastModified.Format("2006-01-02 15:04")
	}
	storageClass := o.StorageClass
	if storageClass == "" {
		storageClass = "-"
	}
	return fmt.Sprintf("%s  %s  %s  %s", o.Name, FormatBytes(o.Size), modified, storageClass)
}

func (o S3Object) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s", o.Key, o.Name))
}

type S3ObjectDetail struct {
	Bucket       string
	Key          string
	Size         int64
	LastModified time.Time
	StorageClass string
	ContentType  string
	ETag         string
}

type S3ListResult struct {
	Prefixes []S3Object
	Objects  []S3Object
}

func NormalizeS3Prefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	return strings.TrimSuffix(prefix, "/") + "/"
}

func ParentS3Prefix(prefix string) string {
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return ""
	}
	parent := path.Dir(prefix)
	if parent == "." {
		return ""
	}
	return NormalizeS3Prefix(parent)
}

func S3Breadcrumb(prefix string) string {
	if prefix == "" {
		return "/"
	}
	return "/" + strings.TrimSuffix(prefix, "/")
}
