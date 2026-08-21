package aws

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type mockDynamoDBClient struct {
	listTablesFunc         func(context.Context, *dynamodb.ListTablesInput, ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
	describeTableFunc      func(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
	describeTimeToLiveFunc func(context.Context, *dynamodb.DescribeTimeToLiveInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error)
	getItemFunc            func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

func (m *mockDynamoDBClient) ListTables(ctx context.Context, input *dynamodb.ListTablesInput, opts ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
	if m.listTablesFunc != nil {
		return m.listTablesFunc(ctx, input, opts...)
	}
	return &dynamodb.ListTablesOutput{}, nil
}

func (m *mockDynamoDBClient) DescribeTable(ctx context.Context, input *dynamodb.DescribeTableInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
	if m.describeTableFunc != nil {
		return m.describeTableFunc(ctx, input, opts...)
	}
	return &dynamodb.DescribeTableOutput{}, nil
}

func (m *mockDynamoDBClient) DescribeTimeToLive(ctx context.Context, input *dynamodb.DescribeTimeToLiveInput, opts ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
	if m.describeTimeToLiveFunc != nil {
		return m.describeTimeToLiveFunc(ctx, input, opts...)
	}
	return &dynamodb.DescribeTimeToLiveOutput{}, nil
}

func (m *mockDynamoDBClient) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemFunc != nil {
		return m.getItemFunc(ctx, input, opts...)
	}
	return &dynamodb.GetItemOutput{}, nil
}

func TestListDynamoDBTablesPaginatesSortsAndMapsSummaries(t *testing.T) {
	listCalls := 0
	mock := &mockDynamoDBClient{
		listTablesFunc: func(_ context.Context, input *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
			listCalls++
			if listCalls == 1 {
				if input.ExclusiveStartTableName != nil {
					t.Fatalf("first page should not have a start table, got %q", awssdk.ToString(input.ExclusiveStartTableName))
				}
				return &dynamodb.ListTablesOutput{TableNames: []string{"Beta"}, LastEvaluatedTableName: awssdk.String("Beta")}, nil
			}
			if awssdk.ToString(input.ExclusiveStartTableName) != "Beta" {
				t.Fatalf("expected second page to start after Beta, got %q", awssdk.ToString(input.ExclusiveStartTableName))
			}
			return &dynamodb.ListTablesOutput{TableNames: []string{"alpha"}}, nil
		},
		describeTableFunc: func(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			name := awssdk.ToString(input.TableName)
			desc := &dynamodbtypes.TableDescription{
				TableName:      awssdk.String(name),
				TableArn:       awssdk.String("arn:aws:dynamodb:ap-northeast-2:123456789012:table/" + name),
				TableStatus:    dynamodbtypes.TableStatusActive,
				TableSizeBytes: awssdk.Int64(2048),
				ItemCount:      awssdk.Int64(12),
				AttributeDefinitions: []dynamodbtypes.AttributeDefinition{
					{AttributeName: awssdk.String("pk"), AttributeType: dynamodbtypes.ScalarAttributeTypeS},
					{AttributeName: awssdk.String("sk"), AttributeType: dynamodbtypes.ScalarAttributeTypeN},
				},
				KeySchema: []dynamodbtypes.KeySchemaElement{
					{AttributeName: awssdk.String("sk"), KeyType: dynamodbtypes.KeyTypeRange},
					{AttributeName: awssdk.String("pk"), KeyType: dynamodbtypes.KeyTypeHash},
				},
			}
			if name == "alpha" {
				desc.BillingModeSummary = &dynamodbtypes.BillingModeSummary{BillingMode: dynamodbtypes.BillingModePayPerRequest}
				desc.GlobalSecondaryIndexes = []dynamodbtypes.GlobalSecondaryIndexDescription{{IndexName: awssdk.String("by-status")}}
			} else {
				desc.ProvisionedThroughput = &dynamodbtypes.ProvisionedThroughputDescription{ReadCapacityUnits: awssdk.Int64(5), WriteCapacityUnits: awssdk.Int64(2)}
			}
			return &dynamodb.DescribeTableOutput{Table: desc}, nil
		},
	}

	repo := &AwsRepository{DynamoDBClient: mock, Region: "ap-northeast-2"}
	tables, err := repo.ListDynamoDBTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listCalls != 2 || len(tables) != 2 {
		t.Fatalf("expected two pages and two tables, calls=%d tables=%d", listCalls, len(tables))
	}
	if tables[0].Name != "alpha" || tables[0].BillingMode != "PAY_PER_REQUEST" || tables[0].GSICount != 1 {
		t.Fatalf("unexpected on-demand table summary: %+v", tables[0])
	}
	if tables[0].Keys[0].Name != "pk" || tables[0].Keys[0].Role != "PARTITION" || tables[0].Keys[1].AttributeType != "N" {
		t.Fatalf("expected ordered typed primary keys, got %+v", tables[0].Keys)
	}
	if tables[1].CapacityLabel() != "5R/2W" || tables[1].Region != "ap-northeast-2" {
		t.Fatalf("unexpected provisioned table summary: %+v", tables[1])
	}
}

func TestDynamoDBMetadataOrderingUsesRawNameTieBreakers(t *testing.T) {
	mock := &mockDynamoDBClient{
		listTablesFunc: func(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{TableNames: []string{"alpha", "Alpha"}}, nil
		},
		describeTableFunc: func(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			name := awssdk.ToString(input.TableName)
			return &dynamodb.DescribeTableOutput{Table: &dynamodbtypes.TableDescription{
				TableName: awssdk.String(name),
				GlobalSecondaryIndexes: []dynamodbtypes.GlobalSecondaryIndexDescription{
					{IndexName: awssdk.String("beta")},
					{IndexName: awssdk.String("alpha")},
					{IndexName: awssdk.String("Alpha")},
				},
			}}, nil
		},
	}

	tables, err := (&AwsRepository{DynamoDBClient: mock}).ListDynamoDBTables(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tables[0].Name != "Alpha" || tables[1].Name != "alpha" {
		t.Fatalf("expected deterministic table order, got %q then %q", tables[0].Name, tables[1].Name)
	}
	gotIndexes := []string{tables[0].GSIs[0].Name, tables[0].GSIs[1].Name, tables[0].GSIs[2].Name}
	if strings.Join(gotIndexes, ",") != "Alpha,alpha,beta" {
		t.Fatalf("expected deterministic GSI order, got %v", gotIndexes)
	}
}

func TestListDynamoDBTablesDescribesTablesConcurrently(t *testing.T) {
	names := make([]string, 9)
	for index := range names {
		names[index] = fmt.Sprintf("table-%02d", index)
	}
	started := make(chan struct{}, len(names))
	release := make(chan struct{})
	mock := &mockDynamoDBClient{
		listTablesFunc: func(context.Context, *dynamodb.ListTablesInput, ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
			return &dynamodb.ListTablesOutput{TableNames: names}, nil
		},
		describeTableFunc: func(_ context.Context, input *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			started <- struct{}{}
			<-release
			return &dynamodb.DescribeTableOutput{Table: &dynamodbtypes.TableDescription{TableName: input.TableName}}, nil
		},
	}

	done := make(chan error, 1)
	go func() {
		_, err := (&AwsRepository{DynamoDBClient: mock}).ListDynamoDBTables(context.Background())
		done <- err
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("expected at least two concurrent DescribeTable calls")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDescribeDynamoDBTableAddsTTLAndStreamDetails(t *testing.T) {
	mock := &mockDynamoDBClient{
		describeTableFunc: func(_ context.Context, _ *dynamodb.DescribeTableInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
			return &dynamodb.DescribeTableOutput{Table: &dynamodbtypes.TableDescription{
				TableName: awssdk.String("orders"),
				StreamSpecification: &dynamodbtypes.StreamSpecification{
					StreamEnabled:  awssdk.Bool(true),
					StreamViewType: dynamodbtypes.StreamViewTypeNewAndOldImages,
				},
				LatestStreamArn: awssdk.String("arn:stream"),
			}}, nil
		},
		describeTimeToLiveFunc: func(_ context.Context, input *dynamodb.DescribeTimeToLiveInput, _ ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
			if awssdk.ToString(input.TableName) != "orders" {
				t.Fatalf("unexpected table name %q", awssdk.ToString(input.TableName))
			}
			return &dynamodb.DescribeTimeToLiveOutput{TimeToLiveDescription: &dynamodbtypes.TimeToLiveDescription{
				AttributeName:    awssdk.String("expires_at"),
				TimeToLiveStatus: dynamodbtypes.TimeToLiveStatusEnabled,
			}}, nil
		},
	}

	table, err := (&AwsRepository{DynamoDBClient: mock}).DescribeDynamoDBTable(context.Background(), "orders")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if table.TTLStatus != "ENABLED" || table.TTLAttribute != "expires_at" || !table.StreamEnabled || table.StreamView != "NEW_AND_OLD_IMAGES" {
		t.Fatalf("unexpected detail: %+v", table)
	}
}

func TestGetDynamoDBItemUsesCompleteTypedKeyAndFormatsJSON(t *testing.T) {
	mock := &mockDynamoDBClient{
		getItemFunc: func(_ context.Context, input *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
			pk, ok := input.Key["tenant"].(*dynamodbtypes.AttributeValueMemberS)
			if !ok || pk.Value != "acme" {
				t.Fatalf("unexpected partition key: %#v", input.Key["tenant"])
			}
			sk, ok := input.Key["order"].(*dynamodbtypes.AttributeValueMemberN)
			if !ok || sk.Value != "42" {
				t.Fatalf("unexpected sort key: %#v", input.Key["order"])
			}
			return &dynamodb.GetItemOutput{Item: map[string]dynamodbtypes.AttributeValue{
				"tenant": &dynamodbtypes.AttributeValueMemberS{Value: "acme"},
				"total":  &dynamodbtypes.AttributeValueMemberN{Value: "19.95"},
				"meta": &dynamodbtypes.AttributeValueMemberM{Value: map[string]dynamodbtypes.AttributeValue{
					"paid": &dynamodbtypes.AttributeValueMemberBOOL{Value: true},
				}},
			}}, nil
		},
	}
	table := DynamoDBTable{Name: "orders", Keys: []DynamoDBKey{
		{Name: "tenant", Role: "PARTITION", AttributeType: "S"},
		{Name: "order", Role: "SORT", AttributeType: "N"},
	}}

	item, err := (&AwsRepository{DynamoDBClient: mock}).GetDynamoDBItem(context.Background(), table, []string{"acme", "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !item.Found || !strings.Contains(item.JSON, `"total": 19.95`) || !strings.Contains(item.JSON, `"paid": true`) {
		t.Fatalf("unexpected item JSON: %s", item.JSON)
	}
}

func TestGetDynamoDBItemRejectsInvalidNumberBeforeAPI(t *testing.T) {
	called := false
	mock := &mockDynamoDBClient{getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		called = true
		return nil, errors.New("should not be called")
	}}
	table := DynamoDBTable{Name: "orders", Keys: []DynamoDBKey{{Name: "id", Role: "PARTITION", AttributeType: "N"}}}

	_, err := (&AwsRepository{DynamoDBClient: mock}).GetDynamoDBItem(context.Background(), table, []string{"not-a-number"})
	if err == nil || called {
		t.Fatalf("expected local validation error without API call, err=%v called=%v", err, called)
	}
}

func TestDynamoDBNumberValidationEnforcesServiceLimits(t *testing.T) {
	key := DynamoDBKey{Name: "id", Role: "PARTITION", AttributeType: "N"}
	for _, value := range []string{
		"0",
		"1e-130",
		"9." + strings.Repeat("9", 37) + "e125",
		"-1e-130",
	} {
		if _, err := newDynamoDBKeyValue(key, value); err != nil {
			t.Errorf("expected %q to be valid: %v", value, err)
		}
	}
	for _, value := range []string{
		"1e-131",
		"1e126",
		strings.Repeat("9", 39),
	} {
		if _, err := newDynamoDBKeyValue(key, value); err == nil {
			t.Errorf("expected %q to be rejected", value)
		}
	}
}

func TestGetDynamoDBItemReturnsNotFound(t *testing.T) {
	mock := &mockDynamoDBClient{getItemFunc: func(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
		return &dynamodb.GetItemOutput{}, nil
	}}
	table := DynamoDBTable{Name: "orders", Keys: []DynamoDBKey{{Name: "id", Role: "PARTITION", AttributeType: "S"}}}

	item, err := (&AwsRepository{DynamoDBClient: mock}).GetDynamoDBItem(context.Background(), table, []string{"missing"})
	if err != nil || item.Found {
		t.Fatalf("expected a clean not-found result, item=%+v err=%v", item, err)
	}
}

func TestListDynamoDBTablesWrapsListError(t *testing.T) {
	mock := &mockDynamoDBClient{listTablesFunc: func(_ context.Context, _ *dynamodb.ListTablesInput, _ ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error) {
		return nil, errors.New("access denied")
	}}
	_, err := (&AwsRepository{DynamoDBClient: mock}).ListDynamoDBTables(context.Background())
	if err == nil || !strings.Contains(err.Error(), "failed to list DynamoDB tables") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDynamoDBOperationsWrapAPIErrors(t *testing.T) {
	tests := []struct {
		name string
		want string
		run  func(*AwsRepository) error
		mock *mockDynamoDBClient
	}{
		{
			name: "describe table",
			want: "failed to describe DynamoDB table orders",
			mock: &mockDynamoDBClient{describeTableFunc: func(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
				return nil, errors.New("access denied")
			}},
			run: func(repo *AwsRepository) error {
				_, err := repo.DescribeDynamoDBTable(context.Background(), "orders")
				return err
			},
		},
		{
			name: "describe TTL",
			want: "failed to describe TTL for DynamoDB table orders",
			mock: &mockDynamoDBClient{
				describeTableFunc: func(context.Context, *dynamodb.DescribeTableInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error) {
					return &dynamodb.DescribeTableOutput{Table: &dynamodbtypes.TableDescription{TableName: awssdk.String("orders")}}, nil
				},
				describeTimeToLiveFunc: func(context.Context, *dynamodb.DescribeTimeToLiveInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeTimeToLiveOutput, error) {
					return nil, errors.New("access denied")
				},
			},
			run: func(repo *AwsRepository) error {
				_, err := repo.DescribeDynamoDBTable(context.Background(), "orders")
				return err
			},
		},
		{
			name: "get item",
			want: "failed to get item from DynamoDB table orders",
			mock: &mockDynamoDBClient{getItemFunc: func(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
				return nil, errors.New("access denied")
			}},
			run: func(repo *AwsRepository) error {
				_, err := repo.GetDynamoDBItem(context.Background(), DynamoDBTable{
					Name: "orders",
					Keys: []DynamoDBKey{{Name: "id", Role: "PARTITION", AttributeType: "S"}},
				}, []string{"42"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.run(&AwsRepository{DynamoDBClient: test.mock})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected wrapped error containing %q, got %v", test.want, err)
			}
		})
	}
}
