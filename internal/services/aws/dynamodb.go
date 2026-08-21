package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	uniclog "unic/internal/log"
)

// ListDynamoDBTables returns table summaries in the active region.
func (r *AwsRepository) ListDynamoDBTables(ctx context.Context) ([]DynamoDBTable, error) {
	uniclog.Debug("aws", "ListDynamoDBTables called")

	var names []string
	paginator := dynamodb.NewListTablesPaginator(r.DynamoDBClient, &dynamodb.ListTablesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list DynamoDB tables: %w", err)
		}
		names = append(names, page.TableNames...)
	}
	sort.Slice(names, func(i, j int) bool {
		left, right := normalizedSortKey(names[i]), normalizedSortKey(names[j])
		if left == right {
			return names[i] < names[j]
		}
		return left < right
	})

	described := make([]*DynamoDBTable, len(names))
	describeErrors := make([]error, len(names))
	jobs := make(chan int, len(names))
	for index := range names {
		jobs <- index
	}
	close(jobs)

	var workers sync.WaitGroup
	for range min(len(names), 8) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				name := names[index]
				out, err := r.DynamoDBClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: awssdk.String(name)})
				if err != nil {
					describeErrors[index] = fmt.Errorf("failed to describe DynamoDB table %s: %w", name, err)
					continue
				}
				if out.Table != nil {
					table := newDynamoDBTable(*out.Table, r.Region)
					described[index] = &table
				}
			}
		}()
	}
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("failed to describe DynamoDB tables: %w", err)
	}

	tables := make([]DynamoDBTable, 0, len(names))
	for index := range names {
		if describeErrors[index] != nil {
			return nil, describeErrors[index]
		}
		if described[index] != nil {
			tables = append(tables, *described[index])
		}
	}
	return tables, nil
}

// DescribeDynamoDBTable returns refreshed table metadata including TTL state.
func (r *AwsRepository) DescribeDynamoDBTable(ctx context.Context, tableName string) (*DynamoDBTable, error) {
	out, err := r.DynamoDBClient.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: awssdk.String(tableName)})
	if err != nil {
		return nil, fmt.Errorf("failed to describe DynamoDB table %s: %w", tableName, err)
	}
	if out.Table == nil {
		return nil, fmt.Errorf("DynamoDB table %s not found", tableName)
	}

	table := newDynamoDBTable(*out.Table, r.Region)
	ttl, err := r.DynamoDBClient.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{TableName: awssdk.String(tableName)})
	if err != nil {
		return nil, fmt.Errorf("failed to describe TTL for DynamoDB table %s: %w", tableName, err)
	}
	if ttl.TimeToLiveDescription != nil {
		table.TTLStatus = string(ttl.TimeToLiveDescription.TimeToLiveStatus)
		table.TTLAttribute = awssdk.ToString(ttl.TimeToLiveDescription.AttributeName)
	}
	return &table, nil
}

// GetDynamoDBItem retrieves at most one item using the selected table's full primary key.
func (r *AwsRepository) GetDynamoDBItem(ctx context.Context, table DynamoDBTable, values []string) (*DynamoDBItem, error) {
	if len(table.Keys) == 0 || len(values) != len(table.Keys) {
		return nil, fmt.Errorf("table %s requires %d key value(s)", table.Name, len(table.Keys))
	}

	key := make(map[string]dynamodbtypes.AttributeValue, len(table.Keys))
	for i, schemaKey := range table.Keys {
		value, err := newDynamoDBKeyValue(schemaKey, values[i])
		if err != nil {
			return nil, err
		}
		key[schemaKey.Name] = value
	}

	out, err := r.DynamoDBClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: awssdk.String(table.Name),
		Key:       key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get item from DynamoDB table %s: %w", table.Name, err)
	}
	if len(out.Item) == 0 {
		return &DynamoDBItem{}, nil
	}

	decoded := make(map[string]any, len(out.Item))
	for name, value := range out.Item {
		decoded[name] = dynamoDBAttributeValue(value)
	}
	payload, err := json.MarshalIndent(decoded, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to format item from DynamoDB table %s: %w", table.Name, err)
	}
	return &DynamoDBItem{Found: true, JSON: string(payload)}, nil
}

func newDynamoDBTable(desc dynamodbtypes.TableDescription, region string) DynamoDBTable {
	attributeTypes := make(map[string]string, len(desc.AttributeDefinitions))
	for _, attribute := range desc.AttributeDefinitions {
		attributeTypes[awssdk.ToString(attribute.AttributeName)] = string(attribute.AttributeType)
	}

	billingMode := "PROVISIONED"
	if desc.BillingModeSummary != nil && desc.BillingModeSummary.BillingMode != "" {
		billingMode = string(desc.BillingModeSummary.BillingMode)
	}
	table := DynamoDBTable{
		Name:          awssdk.ToString(desc.TableName),
		Region:        region,
		ARN:           awssdk.ToString(desc.TableArn),
		Status:        string(desc.TableStatus),
		BillingMode:   billingMode,
		ItemCount:     awssdk.ToInt64(desc.ItemCount),
		SizeBytes:     awssdk.ToInt64(desc.TableSizeBytes),
		GSICount:      len(desc.GlobalSecondaryIndexes),
		CreatedAt:     awssdk.ToTime(desc.CreationDateTime),
		Keys:          newDynamoDBKeys(desc.KeySchema, attributeTypes),
		StreamARN:     awssdk.ToString(desc.LatestStreamArn),
		StreamEnabled: desc.StreamSpecification != nil && awssdk.ToBool(desc.StreamSpecification.StreamEnabled),
	}
	if desc.ProvisionedThroughput != nil {
		table.ReadCapacity = awssdk.ToInt64(desc.ProvisionedThroughput.ReadCapacityUnits)
		table.WriteCapacity = awssdk.ToInt64(desc.ProvisionedThroughput.WriteCapacityUnits)
	}
	if desc.StreamSpecification != nil {
		table.StreamView = string(desc.StreamSpecification.StreamViewType)
	}
	for _, index := range desc.GlobalSecondaryIndexes {
		gsi := DynamoDBGSI{
			Name:   awssdk.ToString(index.IndexName),
			Status: string(index.IndexStatus),
			Keys:   newDynamoDBKeys(index.KeySchema, attributeTypes),
		}
		if index.Projection != nil {
			gsi.Projection = string(index.Projection.ProjectionType)
		}
		if index.ProvisionedThroughput != nil {
			gsi.ReadCapacity = awssdk.ToInt64(index.ProvisionedThroughput.ReadCapacityUnits)
			gsi.WriteCapacity = awssdk.ToInt64(index.ProvisionedThroughput.WriteCapacityUnits)
		}
		table.GSIs = append(table.GSIs, gsi)
	}
	sort.Slice(table.GSIs, func(i, j int) bool {
		left, right := normalizedSortKey(table.GSIs[i].Name), normalizedSortKey(table.GSIs[j].Name)
		if left == right {
			return table.GSIs[i].Name < table.GSIs[j].Name
		}
		return left < right
	})
	return table
}

func newDynamoDBKeys(schema []dynamodbtypes.KeySchemaElement, attributeTypes map[string]string) []DynamoDBKey {
	keys := make([]DynamoDBKey, 0, len(schema))
	for _, element := range schema {
		name := awssdk.ToString(element.AttributeName)
		role := "SORT"
		if element.KeyType == dynamodbtypes.KeyTypeHash {
			role = "PARTITION"
		}
		keys = append(keys, DynamoDBKey{Name: name, Role: role, AttributeType: attributeTypes[name]})
	}
	sort.SliceStable(keys, func(i, j int) bool { return keys[i].Role == "PARTITION" && keys[j].Role != "PARTITION" })
	return keys
}

func newDynamoDBKeyValue(key DynamoDBKey, value string) (dynamodbtypes.AttributeValue, error) {
	if value == "" {
		return nil, fmt.Errorf("%s key cannot be empty", strings.ToLower(key.Role))
	}
	switch key.AttributeType {
	case string(dynamodbtypes.ScalarAttributeTypeS):
		return &dynamodbtypes.AttributeValueMemberS{Value: value}, nil
	case string(dynamodbtypes.ScalarAttributeTypeN):
		number := strings.TrimSpace(value)
		if !validDynamoDBNumber(number) {
			return nil, fmt.Errorf("%s key must be a number", strings.ToLower(key.Role))
		}
		return &dynamodbtypes.AttributeValueMemberN{Value: number}, nil
	case string(dynamodbtypes.ScalarAttributeTypeB):
		binary, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
		if err != nil || len(binary) == 0 {
			return nil, fmt.Errorf("%s key must be non-empty base64", strings.ToLower(key.Role))
		}
		return &dynamodbtypes.AttributeValueMemberB{Value: binary}, nil
	default:
		return nil, fmt.Errorf("unsupported DynamoDB %s key type %q", strings.ToLower(key.Role), key.AttributeType)
	}
}

func validDynamoDBNumber(number string) bool {
	if number == "" {
		return false
	}
	if number[0] == '+' || number[0] == '-' {
		number = number[1:]
		if number == "" {
			return false
		}
	}

	mantissa, exponentText, hasExponent := number, "", false
	if index := strings.IndexAny(number, "eE"); index >= 0 {
		if strings.IndexAny(number[index+1:], "eE") >= 0 {
			return false
		}
		mantissa, exponentText, hasExponent = number[:index], number[index+1:], true
	}
	exponent := int64(0)
	if hasExponent {
		var err error
		exponent, err = strconv.ParseInt(exponentText, 10, 64)
		if err != nil {
			return false
		}
	}

	integer, fraction := mantissa, ""
	if index := strings.IndexByte(mantissa, '.'); index >= 0 {
		if strings.IndexByte(mantissa[index+1:], '.') >= 0 {
			return false
		}
		integer, fraction = mantissa[:index], mantissa[index+1:]
	}
	digits := integer + fraction
	if digits == "" {
		return false
	}
	for _, digit := range digits {
		if digit < '0' || digit > '9' {
			return false
		}
	}

	firstNonZero := strings.IndexAny(digits, "123456789")
	if firstNonZero < 0 {
		return true
	}
	significant := strings.TrimRight(digits[firstNonZero:], "0")
	if len(significant) > 38 {
		return false
	}
	adjustment := int64(len(integer) - firstNonZero - 1)
	return exponent >= -130-adjustment && exponent <= 125-adjustment
}

func dynamoDBAttributeValue(value dynamodbtypes.AttributeValue) any {
	switch value := value.(type) {
	case *dynamodbtypes.AttributeValueMemberS:
		return value.Value
	case *dynamodbtypes.AttributeValueMemberN:
		return json.Number(value.Value)
	case *dynamodbtypes.AttributeValueMemberB:
		return base64.StdEncoding.EncodeToString(value.Value)
	case *dynamodbtypes.AttributeValueMemberBOOL:
		return value.Value
	case *dynamodbtypes.AttributeValueMemberNULL:
		return nil
	case *dynamodbtypes.AttributeValueMemberSS:
		return value.Value
	case *dynamodbtypes.AttributeValueMemberNS:
		values := make([]json.Number, len(value.Value))
		for i, number := range value.Value {
			values[i] = json.Number(number)
		}
		return values
	case *dynamodbtypes.AttributeValueMemberBS:
		values := make([]string, len(value.Value))
		for i, binary := range value.Value {
			values[i] = base64.StdEncoding.EncodeToString(binary)
		}
		return values
	case *dynamodbtypes.AttributeValueMemberL:
		values := make([]any, len(value.Value))
		for i, item := range value.Value {
			values[i] = dynamoDBAttributeValue(item)
		}
		return values
	case *dynamodbtypes.AttributeValueMemberM:
		values := make(map[string]any, len(value.Value))
		for name, item := range value.Value {
			values[name] = dynamoDBAttributeValue(item)
		}
		return values
	default:
		return fmt.Sprintf("<unsupported %T>", value)
	}
}
