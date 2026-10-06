package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/androsh7/cloud_command/internal/config"
	"github.com/androsh7/cloud_command/internal/control"
	"github.com/androsh7/cloud_command/pkg/agent"
)

// connect wires a server for an empty store to an in-memory client session.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := New(control.New(agent.NewStore(t.TempDir()), config.DefaultSettings(), nil, "test"))
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestServerListsEveryTool(t *testing.T) {
	session := connect(t)

	listed, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	names := map[string]*mcp.Tool{}
	for _, tool := range listed.Tools {
		names[tool.Name] = tool
	}
	for _, expected := range []string{"list_agents", "get_agent", "create_agent", "terminate_agent", "run_command", "start_job", "job_status", "stop_job", "remove_job"} {
		if names[expected] == nil {
			t.Fatalf("tool %q missing from %v", expected, names)
		}
	}
	if !names["list_agents"].Annotations.ReadOnlyHint || names["terminate_agent"].Annotations.DestructiveHint == nil {
		t.Fatal("annotations missing")
	}
}

func TestListAgentsOnEmptyStoreReturnsEmptyList(t *testing.T) {
	session := connect(t)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_agents", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool reported an error: %+v", result.Content)
	}
	encoded, _ := json.Marshal(result.StructuredContent)
	var output ListAgentsOutput
	if err := json.Unmarshal(encoded, &output); err != nil {
		t.Fatalf("decoding structured content: %v", err)
	}
	if len(output.Agents) != 0 {
		t.Fatalf("got %d agents", len(output.Agents))
	}
}

func TestGetAgentForUnknownNameIsToolError(t *testing.T) {
	session := connect(t)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_agent", Arguments: map[string]any{"name": "ghost"}})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected a tool error for an unknown agent")
	}
}

func TestCreateAgentRejectsBadNameAsToolError(t *testing.T) {
	session := connect(t)

	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_agent", Arguments: map[string]any{"name": "../x"}})
	if err != nil {
		t.Fatalf("CallTool returned protocol error: %v", err)
	}

	if !result.IsError {
		t.Fatal("expected a tool error for an invalid name")
	}
}
