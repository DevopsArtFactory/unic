package aws

import (
	"context"
	"fmt"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// mockCloudWatchLogsClient implements CloudWatchLogsClientAPI for testing.
type mockCloudWatchLogsClient struct {
	describeLogGroupsFunc  func(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	describeLogStreamsFunc func(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	filterLogEventsFunc    func(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

func (m *mockCloudWatchLogsClient) DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	return m.describeLogGroupsFunc(ctx, params, optFns...)
}

func (m *mockCloudWatchLogsClient) DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	return m.describeLogStreamsFunc(ctx, params, optFns...)
}

func (m *mockCloudWatchLogsClient) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	if m.filterLogEventsFunc != nil {
		return m.filterLogEventsFunc(ctx, params, optFns...)
	}
	return nil, fmt.Errorf("not implemented")
}

// --- ListLogGroups tests ---

func TestListLogGroups_Success(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogGroupsFunc: func(_ context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			if awssdk.ToInt32(params.Limit) != cwLogGroupPageSize {
				t.Fatalf("expected limit %d, got %d", cwLogGroupPageSize, awssdk.ToInt32(params.Limit))
			}
			var retention int32 = 30
			var creationTime int64 = 1700000000000
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwltypes.LogGroup{
					{
						LogGroupName:    awssdk.String("/aws/lambda/my-function"),
						Arn:             awssdk.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-function:*"),
						RetentionInDays: &retention,
						StoredBytes:     awssdk.Int64(1048576),
						CreationTime:    &creationTime,
					},
					{
						LogGroupName: awssdk.String("/ecs/my-service"),
						Arn:          awssdk.String("arn:aws:logs:us-east-1:123456789012:log-group:/ecs/my-service:*"),
						StoredBytes:  awssdk.Int64(0),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	groups, err := repo.ListLogGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	g := groups[0]
	if g.Name != "/aws/lambda/my-function" {
		t.Errorf("expected name '/aws/lambda/my-function', got %q", g.Name)
	}
	if g.RetentionDays != 30 {
		t.Errorf("expected retention 30, got %d", g.RetentionDays)
	}
	if g.StoredBytes != 1048576 {
		t.Errorf("expected stored bytes 1048576, got %d", g.StoredBytes)
	}

	g2 := groups[1]
	if g2.RetentionDays != 0 {
		t.Errorf("expected retention 0 (never expire), got %d", g2.RetentionDays)
	}
}

func TestListLogGroups_Empty(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogGroupsFunc: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwltypes.LogGroup{},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	groups, err := repo.ListLogGroups(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected empty slice, got %d", len(groups))
	}
}

func TestListLogGroups_Error(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogGroupsFunc: func(_ context.Context, _ *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			return nil, fmt.Errorf("access denied")
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	_, err := repo.ListLogGroups(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListLogGroupsPage_WithToken(t *testing.T) {
	token := "next-page"
	mock := &mockCloudWatchLogsClient{
		describeLogGroupsFunc: func(_ context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
			if got := awssdk.ToString(params.NextToken); got != token {
				t.Fatalf("expected token %q, got %q", token, got)
			}
			if awssdk.ToInt32(params.Limit) != 10 {
				t.Fatalf("expected limit 10, got %d", awssdk.ToInt32(params.Limit))
			}
			return &cloudwatchlogs.DescribeLogGroupsOutput{
				LogGroups: []cwltypes.LogGroup{
					{LogGroupName: awssdk.String("/aws/lambda/page-2")},
				},
				NextToken: awssdk.String("page-3"),
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	groups, next, err := repo.ListLogGroupsPage(context.Background(), &token, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Name != "/aws/lambda/page-2" {
		t.Fatalf("expected page-2 group, got %q", groups[0].Name)
	}
	if next == nil || *next != "page-3" {
		t.Fatalf("expected next token page-3, got %v", next)
	}
}

// --- ListLogStreams tests ---

func TestListLogStreams_Success(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogStreamsFunc: func(_ context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if awssdk.ToString(params.LogGroupName) != "/aws/lambda/my-function" {
				t.Errorf("expected log group '/aws/lambda/my-function', got %q", awssdk.ToString(params.LogGroupName))
			}
			if awssdk.ToInt32(params.Limit) != cwLogStreamPageSize {
				t.Fatalf("expected limit %d, got %d", cwLogStreamPageSize, awssdk.ToInt32(params.Limit))
			}
			var lastEvent int64 = 1700000000000
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{
					{
						LogStreamName:      awssdk.String("2024/01/01/[$LATEST]abc123"),
						LastEventTimestamp: &lastEvent,
					},
					{
						LogStreamName: awssdk.String("2024/01/01/[$LATEST]def456"),
					},
				},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	streams, err := repo.ListLogStreams(context.Background(), "/aws/lambda/my-function")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(streams))
	}

	s := streams[0]
	if s.Name != "2024/01/01/[$LATEST]abc123" {
		t.Errorf("expected name '2024/01/01/[$LATEST]abc123', got %q", s.Name)
	}
	if s.LastEventTime.IsZero() {
		t.Error("expected non-zero last event time")
	}
}

func TestListLogStreams_Empty(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogStreamsFunc: func(_ context.Context, _ *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	streams, err := repo.ListLogStreams(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 0 {
		t.Errorf("expected empty slice, got %d", len(streams))
	}
}

func TestListLogStreams_Error(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		describeLogStreamsFunc: func(_ context.Context, _ *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			return nil, fmt.Errorf("log group not found")
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	_, err := repo.ListLogStreams(context.Background(), "/nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListLogStreamsPage_WithToken(t *testing.T) {
	token := "stream-page-2"
	mock := &mockCloudWatchLogsClient{
		describeLogStreamsFunc: func(_ context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
			if got := awssdk.ToString(params.NextToken); got != token {
				t.Fatalf("expected token %q, got %q", token, got)
			}
			if awssdk.ToInt32(params.Limit) != 10 {
				t.Fatalf("expected limit 10, got %d", awssdk.ToInt32(params.Limit))
			}
			return &cloudwatchlogs.DescribeLogStreamsOutput{
				LogStreams: []cwltypes.LogStream{
					{LogStreamName: awssdk.String("page-2-stream")},
				},
				NextToken: awssdk.String("stream-page-3"),
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	streams, next, err := repo.ListLogStreamsPage(context.Background(), "/aws/lambda/my-function", &token, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(streams) != 1 {
		t.Fatalf("expected 1 stream, got %d", len(streams))
	}
	if streams[0].Name != "page-2-stream" {
		t.Fatalf("expected page-2-stream, got %q", streams[0].Name)
	}
	if next == nil || *next != "stream-page-3" {
		t.Fatalf("expected next token stream-page-3, got %v", next)
	}
}

// --- FilterLogEvents tests ---

func TestFilterLogEvents_Success(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		filterLogEventsFunc: func(_ context.Context, params *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
			if awssdk.ToString(params.LogGroupName) != "/aws/lambda/my-function" {
				t.Errorf("expected log group name, got %q", awssdk.ToString(params.LogGroupName))
			}
			var ts int64 = 1700000000000
			return &cloudwatchlogs.FilterLogEventsOutput{
				Events: []cwltypes.FilteredLogEvent{
					{
						EventId:   awssdk.String("evt-1"),
						Timestamp: &ts,
						Message:   awssdk.String("2024-01-01T00:00:00Z INFO Starting handler"),
					},
					{
						EventId:   awssdk.String("evt-2"),
						Timestamp: &ts,
						Message:   awssdk.String("2024-01-01T00:00:01Z ERROR Something failed"),
					},
				},
				NextToken: awssdk.String("next-page-token"),
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	events, nextToken, err := repo.FilterLogEvents(context.Background(), "/aws/lambda/my-function", 1700000000000, 1700000060000, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Level != "INFO" {
		t.Errorf("expected level 'INFO', got %q", events[0].Level)
	}
	if events[0].EventID != "evt-1" {
		t.Errorf("expected event ID 'evt-1', got %q", events[0].EventID)
	}
	if events[1].Level != "ERROR" {
		t.Errorf("expected level 'ERROR', got %q", events[1].Level)
	}
	if events[1].EventID != "evt-2" {
		t.Errorf("expected event ID 'evt-2', got %q", events[1].EventID)
	}
	if nextToken == nil {
		t.Error("expected non-nil next token")
	}
}

func TestFilterLogEvents_Empty(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		filterLogEventsFunc: func(_ context.Context, _ *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
			return &cloudwatchlogs.FilterLogEventsOutput{
				Events: []cwltypes.FilteredLogEvent{},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	events, _, err := repo.FilterLogEvents(context.Background(), "/test", 0, 0, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected empty slice, got %d", len(events))
	}
}

func TestFilterLogEvents_Error(t *testing.T) {
	mock := &mockCloudWatchLogsClient{
		filterLogEventsFunc: func(_ context.Context, _ *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
			return nil, fmt.Errorf("throttled")
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	_, _, err := repo.FilterLogEvents(context.Background(), "/test", 0, 0, "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFilterLogEvents_WithToken(t *testing.T) {
	token := "page-2-token"
	mock := &mockCloudWatchLogsClient{
		filterLogEventsFunc: func(_ context.Context, params *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
			if awssdk.ToString(params.NextToken) != token {
				t.Errorf("expected token %q, got %q", token, awssdk.ToString(params.NextToken))
			}
			return &cloudwatchlogs.FilterLogEventsOutput{
				Events: []cwltypes.FilteredLogEvent{},
			}, nil
		},
	}

	repo := &AwsRepository{CloudWatchLogsClient: mock}
	_, _, err := repo.FilterLogEvents(context.Background(), "/test", 0, 0, "", &token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Model tests ---

func TestLogGroupDisplayTitle(t *testing.T) {
	g := LogGroup{Name: "/aws/lambda/my-func", RetentionDays: 30, StoredBytes: 1048576}
	title := g.DisplayTitle()
	if !strings.Contains(title, "/aws/lambda/my-func") {
		t.Errorf("DisplayTitle should contain name, got %q", title)
	}
	if !strings.Contains(title, "30 days") {
		t.Errorf("DisplayTitle should contain retention, got %q", title)
	}
	if !strings.Contains(title, "1.0 MB") {
		t.Errorf("DisplayTitle should contain size, got %q", title)
	}
}

func TestLogGroupDisplayTitle_NeverExpire(t *testing.T) {
	g := LogGroup{Name: "/test", RetentionDays: 0, StoredBytes: 0}
	title := g.DisplayTitle()
	if !strings.Contains(title, "Never expire") {
		t.Errorf("DisplayTitle should say 'Never expire' for 0 retention, got %q", title)
	}
}

func TestLogGroupFilterText(t *testing.T) {
	g := LogGroup{Name: "/AWS/Lambda/MyFunc"}
	ft := g.FilterText()
	if !strings.Contains(ft, "/aws/lambda/myfunc") {
		t.Errorf("FilterText should be lowercase, got %q", ft)
	}
}

func TestLogStreamDisplayTitle(t *testing.T) {
	s := LogStream{Name: "stream-1"}
	title := s.DisplayTitle()
	if !strings.Contains(title, "stream-1") {
		t.Errorf("DisplayTitle should contain name, got %q", title)
	}
	if !strings.Contains(title, "No events") {
		t.Errorf("DisplayTitle should say 'No events' for zero time, got %q", title)
	}
}

func TestLogStreamFilterText(t *testing.T) {
	s := LogStream{Name: "MyStream"}
	ft := s.FilterText()
	if ft != "mystream" {
		t.Errorf("FilterText should be lowercase, got %q", ft)
	}
}

func TestLogEventDisplayTitle(t *testing.T) {
	e := LogEvent{Message: "INFO something happened", Level: "INFO"}
	title := e.DisplayTitle()
	if !strings.Contains(title, "[INFO]") {
		t.Errorf("DisplayTitle should contain level, got %q", title)
	}
}

func TestExtractLogLevel(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"2024-01-01 INFO Starting", "INFO"},
		{"ERROR: something broke", "ERROR"},
		{"[WARN] low memory", "WARN"},
		{"DEBUG trace message", "DEBUG"},
		{"FATAL crash", "FATAL"},
		{"just a plain message", ""},
	}
	for _, tt := range tests {
		got := extractLogLevel(tt.message)
		if got != tt.want {
			t.Errorf("extractLogLevel(%q) = %q, want %q", tt.message, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := FormatBytes(tt.input)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
