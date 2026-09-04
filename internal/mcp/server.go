// Package mcp exposes unic's existing automation commands over MCP stdio.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const ProtocolVersion = "2025-11-25"

type Executor func(context.Context, ...string) ([]byte, error)

type Server struct {
	version string
	execute Executor
}

func New(version string, execute Executor) *Server {
	return &Server{version: version, execute: execute}
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	Annotations annotations    `json:"annotations"`
}

type annotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint"`
	DestructiveHint bool `json:"destructiveHint"`
	IdempotentHint  bool `json:"idempotentHint"`
	OpenWorldHint   bool `json:"openWorldHint"`
}

var tools = []tool{
	{
		Name: "get_capabilities", Description: "List unic AWS features and automation commands.",
		InputSchema: objectSchema(nil, nil),
		Annotations: annotations{ReadOnlyHint: true, IdempotentHint: true},
	},
	{
		Name: "get_command_schema", Description: "Describe one unic automation command contract.",
		InputSchema: objectSchema(map[string]any{
			"command": map[string]any{"type": "string", "description": "Command path, for example: context sync"},
		}, []string{"command"}),
		Annotations: annotations{ReadOnlyHint: true, IdempotentHint: true},
	},
	{
		Name: "list_backup_vaults", Description: "List AWS Backup vaults using unic's active or selected AWS context.",
		InputSchema: objectSchema(map[string]any{
			"profile": map[string]any{"type": "string", "description": "Optional unic context or AWS profile"},
			"region":  map[string]any{"type": "string", "description": "Optional AWS region"},
		}, nil),
		Annotations: annotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
	},
	{
		Name: "plan_context_sync", Description: "Preview an SSO context sync plan. This tool never writes configuration.",
		InputSchema: objectSchema(map[string]any{
			"base_context": map[string]any{"type": "string", "description": "Optional SSO base context"},
			"prune":        map[string]any{"type": "boolean", "default": false, "description": "Show orphaned managed contexts as removals"},
		}, nil),
		Annotations: annotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: true},
	},
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	if properties == nil {
		properties = map[string]any{}
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// Serve reads newline-delimited JSON-RPC messages and writes MCP responses.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	encoder := json.NewEncoder(out)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		res, reply := s.handle(ctx, line)
		if reply {
			if err := encoder.Encode(res); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, message []byte) (response, bool) {
	var req request
	if err := json.Unmarshal(message, &req); err != nil {
		return errorResponse(nil, -32700, "Parse error"), true
	}
	if req.JSONRPC != "2.0" || req.Method == "" || (!validID(req.ID) && len(req.ID) > 0) {
		return errorResponse(nil, -32600, "Invalid Request"), true
	}
	if len(req.ID) == 0 {
		return response{}, false
	}

	res := response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		var params struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := decodeParams(req.Params, &params); err != nil || params.ProtocolVersion == "" {
			return errorResponse(req.ID, -32602, "Invalid initialize parameters"), true
		}
		if !supportedProtocol(params.ProtocolVersion) {
			return errorResponse(req.ID, -32602, "Unsupported protocol version"), true
		}
		res.Result = map[string]any{
			"protocolVersion": params.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]string{"name": "unic", "version": s.version},
			"instructions":    "Use unic tools to discover capabilities and query AWS. Context sync is preview-only and never changes configuration.",
		}
	case "ping":
		res.Result = map[string]any{}
	case "tools/list":
		res.Result = map[string]any{"tools": tools}
	case "tools/call":
		result, rpcErr := s.callTool(ctx, req.Params)
		if rpcErr != nil {
			res.Error = rpcErr
		} else {
			res.Result = result
		}
	default:
		res.Error = &rpcError{Code: -32601, Message: "Method not found"}
	}
	return res, true
}

func supportedProtocol(version string) bool {
	switch version {
	case "2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05":
		return true
	default:
		return false
	}
}

func validID(id json.RawMessage) bool {
	if len(id) == 0 {
		return false
	}
	var value any
	if json.Unmarshal(id, &value) != nil {
		return false
	}
	switch value.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

func (s *Server) callTool(ctx context.Context, raw json.RawMessage) (any, *rpcError) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := decodeParams(raw, &call); err != nil || call.Name == "" {
		return nil, &rpcError{Code: -32602, Message: "Invalid tool call parameters"}
	}

	args, err := toolArgs(call.Name, call.Arguments)
	if err != nil {
		if errors.Is(err, errUnknownTool) {
			return nil, &rpcError{Code: -32602, Message: err.Error()}
		}
		return toolError(err), nil
	}
	output, executeErr := s.execute(ctx, args...)
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		if executeErr == nil {
			executeErr = errors.New("unic command returned no JSON")
		}
		return toolError(executeErr), nil
	}

	var structured map[string]any
	if err := json.Unmarshal(output, &structured); err != nil {
		return toolError(fmt.Errorf("unic command returned invalid JSON: %w", err)), nil
	}
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": string(output)}},
		"structuredContent": structured,
		"isError":           executeErr != nil,
	}, nil
}

var errUnknownTool = errors.New("unknown tool")

func toolArgs(name string, raw json.RawMessage) ([]string, error) {
	switch name {
	case "get_capabilities":
		if err := decodeArguments(raw, &struct{}{}); err != nil {
			return nil, err
		}
		return []string{"capabilities", "--json"}, nil
	case "get_command_schema":
		var args struct {
			Command string `json:"command"`
		}
		if err := decodeArguments(raw, &args); err != nil {
			return nil, err
		}
		command := strings.Fields(args.Command)
		if len(command) == 0 {
			return nil, errors.New("command is required")
		}
		return append(append([]string{"schema"}, command...), "--json"), nil
	case "list_backup_vaults":
		var args struct {
			Profile string `json:"profile"`
			Region  string `json:"region"`
		}
		if err := decodeArguments(raw, &args); err != nil {
			return nil, err
		}
		result := []string{"resources", "backup-vaults", "--json"}
		if args.Profile != "" {
			result = append(result, "--profile", args.Profile)
		}
		if args.Region != "" {
			result = append(result, "--region", args.Region)
		}
		return result, nil
	case "plan_context_sync":
		var args struct {
			BaseContext string `json:"base_context"`
			Prune       bool   `json:"prune"`
		}
		if err := decodeArguments(raw, &args); err != nil {
			return nil, err
		}
		result := []string{"context", "sync"}
		if args.BaseContext != "" {
			result = append(result, args.BaseContext)
		}
		result = append(result, "--json")
		if args.Prune {
			result = append(result, "--prune")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%w %q", errUnknownTool, name)
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	return json.Unmarshal(raw, target)
}

func decodeArguments(raw json.RawMessage, target any) error {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		raw = []byte("{}")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func toolError(err error) map[string]any {
	structured := map[string]any{"code": "operation_failed", "message": err.Error(), "retryable": false}
	encoded, _ := json.Marshal(structured)
	return map[string]any{
		"content":           []map[string]string{{"type": "text", "text": string(encoded)}},
		"structuredContent": structured,
		"isError":           true,
	}
}

func errorResponse(id json.RawMessage, code int, message string) response {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	return response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}}
}
