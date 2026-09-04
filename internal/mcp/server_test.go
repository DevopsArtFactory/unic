package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestServerLifecycleAndTools(t *testing.T) {
	var calls [][]string
	execute := func(_ context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return []byte(`{"schema_version":"v1","data":[]}`), nil
	}
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_backup_vaults","arguments":{"profile":"prod","region":"us-east-1"}}}`,
	}, "\n")
	var output bytes.Buffer
	if err := New("test", execute).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}

	responses := decodeResponses(t, output.String())
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3: %s", len(responses), output.String())
	}
	if got := responses[0].Result.(map[string]any)["protocolVersion"]; got != ProtocolVersion {
		t.Fatalf("protocolVersion = %v", got)
	}
	listed := responses[1].Result.(map[string]any)["tools"].([]any)
	if len(listed) != 4 {
		t.Fatalf("listed %d tools", len(listed))
	}
	wantCall := []string{"resources", "backup-vaults", "--json", "--profile", "prod", "--region", "us-east-1"}
	if !reflect.DeepEqual(calls, [][]string{wantCall}) {
		t.Fatalf("calls = %#v", calls)
	}
	result := responses[2].Result.(map[string]any)
	if result["isError"] != false || result["structuredContent"].(map[string]any)["schema_version"] != "v1" {
		t.Fatalf("unexpected tool result: %#v", result)
	}
}

func TestPlanContextSyncNeverAddsApply(t *testing.T) {
	args, err := toolArgs("plan_context_sync", json.RawMessage(`{"base_context":"dev","prune":true}`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"context", "sync", "dev", "--json", "--prune"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
	for _, arg := range args {
		if arg == "--apply" {
			t.Fatal("preview-only MCP tool must not apply changes")
		}
	}
}

func TestToolFailureReturnsStructuredToolError(t *testing.T) {
	execute := func(context.Context, ...string) ([]byte, error) { return nil, errors.New("boom") }
	input := `{"jsonrpc":"2.0","id":"x","method":"tools/call","params":{"name":"get_capabilities","arguments":{}}}`
	var output bytes.Buffer
	if err := New("test", execute).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeResponses(t, output.String())
	if len(responses) == 0 {
		t.Fatal("expected a response")
	}
	result := responses[0].Result.(map[string]any)
	if result["isError"] != true {
		t.Fatalf("expected tool error: %#v", result)
	}
	structured := result["structuredContent"].(map[string]any)
	if structured["code"] != "operation_failed" || structured["message"] != "boom" {
		t.Fatalf("unexpected structured error: %#v", structured)
	}
}

func TestServerRejectsMalformedRequest(t *testing.T) {
	var output bytes.Buffer
	if err := New("test", nil).Serve(context.Background(), strings.NewReader("not-json\n"), &output); err != nil {
		t.Fatal(err)
	}
	responses := decodeResponses(t, output.String())
	if len(responses) == 0 {
		t.Fatal("expected a response")
	}
	if responses[0].Error == nil || responses[0].Error.Code != -32700 {
		t.Fatalf("unexpected response: %#v", responses[0])
	}
}

func decodeResponses(t *testing.T, output string) []responseForTest {
	t.Helper()
	var responses []responseForTest
	decoder := json.NewDecoder(strings.NewReader(output))
	for decoder.More() {
		var response responseForTest
		if err := decoder.Decode(&response); err != nil {
			t.Fatal(err)
		}
		responses = append(responses, response)
	}
	return responses
}

type responseForTest struct {
	Result any       `json:"result"`
	Error  *rpcError `json:"error"`
}
