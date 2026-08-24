package aws

import (
	"fmt"
	"time"
)

// LambdaFunction represents an AWS Lambda function.
type LambdaFunction struct {
	Name         string
	Region       string
	ARN          string
	Runtime      string
	Handler      string
	MemoryMB     int32
	TimeoutSec   int32
	CodeSize     int64
	LastModified string
	Description  string
	Role         string
	Layers       []string
	VPCSubnets   []string
	VPCSGs       []string
}

func (f LambdaFunction) DisplayTitle() string {
	return fmt.Sprintf("%-40s  %-14s  %4dMB  %3ds  %s", f.Name, f.Runtime, f.MemoryMB, f.TimeoutSec, f.shortLastModified())
}

func (f LambdaFunction) FilterText() string {
	return f.Name + " " + f.Runtime + " " + f.Region
}

func (f LambdaFunction) shortLastModified() string {
	t, err := time.Parse("2006-01-02T15:04:05.000+0000", f.LastModified)
	if err != nil {
		return f.LastModified
	}
	return t.Format("2006-01-02 15:04")
}

// LambdaInvokeResult holds the result of a Lambda invocation.
type LambdaInvokeResult struct {
	StatusCode    int32
	Payload       string
	FunctionError string
	LogResult     string
}
