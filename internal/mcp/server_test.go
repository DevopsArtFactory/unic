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
	if len(listed) != len(tools) {
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

func TestReadOnlyOperationToolArgs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"list_ec2_instances", `{"profile":"prod","region":"eu-west-1"}`, []string{"resources", "ec2-instances", "--json", "--profile", "prod", "--region", "eu-west-1"}},
		{"get_ecs_service_rollout", `{"cluster":"prod","service":"api"}`, []string{"resources", "ecs-rollout", "--cluster", "prod", "--service", "api", "--json"}},
		{"list_cloudtrail_events", `{"since":"6h","mutations_only":true}`, []string{"resources", "cloudtrail-events", "--since", "6h", "--json", "--mutations-only"}},
		{"get_elb_target_health", `{"load_balancer":"arn:lb"}`, []string{"resources", "elb-target-health", "--load-balancer", "arn:lb", "--json"}},
		{"run_security_inspector", `{}`, []string{"inspect", "--json"}},
		{"run_security_inspector", `{"profile":"prod","region":"eu-west-1"}`, []string{"inspect", "--json", "--profile", "prod", "--region", "eu-west-1"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := toolArgs(test.name, json.RawMessage(test.raw))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("args = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestReadOnlyOperationToolValidation(t *testing.T) {
	if _, err := toolArgs("get_ecs_service_rollout", json.RawMessage(`{"cluster":"prod"}`)); err == nil {
		t.Fatal("missing service must fail")
	}
	if _, err := toolArgs("get_elb_target_health", json.RawMessage(`{"load_balancer":""}`)); err == nil {
		t.Fatal("empty load balancer must fail")
	}
}

func TestMCPCapabilitiesStayAlignedWithRegisteredTools(t *testing.T) {
	capabilities := mcpCapabilities()
	listed := capabilities["tools"].([]map[string]any)
	if len(listed) != len(tools) {
		t.Fatalf("capabilities list %d tools, registration has %d", len(listed), len(tools))
	}
	for i, registered := range tools {
		if listed[i]["name"] != registered.Name {
			t.Fatalf("tool %d = %v, want %s", i, listed[i]["name"], registered.Name)
		}
		if listed[i]["output_contract"] == "" {
			t.Fatalf("tool %s has no output contract", registered.Name)
		}
		if listed[i]["input_contract"] == "" {
			t.Fatalf("tool %s has no input contract", registered.Name)
		}
		if _, ok := listed[i]["required_permissions"].([]string); !ok {
			t.Fatalf("tool %s permissions are not a stable array", registered.Name)
		}
	}
}

func TestGetMCPCapabilitiesDoesNotExecuteCLI(t *testing.T) {
	execute := func(context.Context, ...string) ([]byte, error) {
		t.Fatal("internal capability discovery must not execute the CLI")
		return nil, nil
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_mcp_capabilities","arguments":{}}}`
	var output bytes.Buffer
	if err := New("test", execute).Serve(context.Background(), strings.NewReader(input), &output); err != nil {
		t.Fatal(err)
	}
	result := decodeResponses(t, output.String())[0].Result.(map[string]any)
	structured := result["structuredContent"].(map[string]any)
	if structured["schema_version"] != "v1" || len(structured["tools"].([]any)) != len(tools) {
		t.Fatalf("unexpected capabilities: %#v", structured)
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

func TestSecurityInspectorToolIsReadOnlyAndNotAResourceContract(t *testing.T) {
	var found bool
	for _, registered := range tools {
		if registered.Name != "run_security_inspector" {
			continue
		}
		found = true
		if !registered.Annotations.ReadOnlyHint {
			t.Error("the inspector scan must advertise read-only")
		}
		if registered.Metadata.OutputContract != "unic.inspect.v1" {
			t.Errorf("output contract = %q, want unic.inspect.v1", registered.Metadata.OutputContract)
		}
		// Inspector is a workflow, not a catalog feature, so it must stay out
		// of the unic.resources.* namespace the catalog parity test walks.
		if strings.HasPrefix(registered.Metadata.OutputContract, "unic.resources.") {
			t.Error("the inspector scan must not claim a unic.resources.* contract")
		}
	}
	if !found {
		t.Fatal("run_security_inspector is not registered")
	}
}
