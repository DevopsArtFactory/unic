package aws

import (
	"fmt"
	"strings"
	"time"
)

// DynamoDBKey describes one attribute in a table or index key schema.
type DynamoDBKey struct {
	Name          string
	Role          string
	AttributeType string
}

// DynamoDBGSI describes a global secondary index.
type DynamoDBGSI struct {
	Name          string
	Status        string
	Projection    string
	Keys          []DynamoDBKey
	ReadCapacity  int64
	WriteCapacity int64
}

// DynamoDBTable holds table metadata used by the browser and key lookup flow.
type DynamoDBTable struct {
	Name          string
	Region        string
	ARN           string
	Status        string
	BillingMode   string
	ReadCapacity  int64
	WriteCapacity int64
	ItemCount     int64
	SizeBytes     int64
	GSICount      int
	CreatedAt     time.Time
	Keys          []DynamoDBKey
	GSIs          []DynamoDBGSI
	TTLStatus     string
	TTLAttribute  string
	StreamEnabled bool
	StreamView    string
	StreamARN     string
}

// FilterText returns table metadata used by the shared fuzzy filter.
func (t DynamoDBTable) FilterText() string {
	return strings.ToLower(fmt.Sprintf("%s %s %s %s", t.Name, t.Status, t.BillingMode, t.Region))
}

// CapacityLabel summarizes provisioned throughput or the on-demand mode.
func (t DynamoDBTable) CapacityLabel() string {
	if t.BillingMode == "PAY_PER_REQUEST" {
		return "on-demand"
	}
	return fmt.Sprintf("%dR/%dW", t.ReadCapacity, t.WriteCapacity)
}

// DynamoDBItem is the bounded result of a single GetItem lookup.
type DynamoDBItem struct {
	Found bool
	JSON  string
}
