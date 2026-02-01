package middleware

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestInput is a sample input struct for testing
type TestInput struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// TestOutput is a sample output struct for testing
type TestOutput struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
}

func TestTracedTool_Success(t *testing.T) {
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		return nil, TestOutput{Result: "hello " + input.Name, Success: true}, nil
	}

	wrapped := TracedTool("test_tool", handler)

	ctx := context.Background()
	input := TestInput{Name: "world", Value: 42}

	result, output, err := wrapped(ctx, nil, input)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
	if output.Result != "hello world" {
		t.Errorf("result = %q, want %q", output.Result, "hello world")
	}
	if !output.Success {
		t.Error("expected success = true")
	}
}

func TestTracedTool_Error(t *testing.T) {
	expectedErr := errors.New("tool failed")
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		return nil, TestOutput{}, expectedErr
	}

	wrapped := TracedTool("failing_tool", handler)

	ctx := context.Background()
	input := TestInput{Name: "test", Value: 1}

	_, _, err := wrapped(ctx, nil, input)

	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestTracedTool_WithRequest(t *testing.T) {
	var receivedReq *mcp.CallToolRequest
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		receivedReq = req
		return nil, TestOutput{Result: "ok", Success: true}, nil
	}

	wrapped := TracedTool("request_tool", handler)

	ctx := context.Background()
	input := TestInput{Name: "test", Value: 1}

	// Pass nil request - handler should still work
	_, output, err := wrapped(ctx, nil, input)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if receivedReq != nil {
		t.Error("expected nil request to be passed")
	}
	if output.Result != "ok" {
		t.Errorf("result = %q, want %q", output.Result, "ok")
	}
}

func TestTracedTool_ContextPropagation(t *testing.T) {
	type ctxKey string
	key := ctxKey("test-key")
	expectedValue := "test-value"

	var receivedValue string
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		if v, ok := ctx.Value(key).(string); ok {
			receivedValue = v
		}
		return nil, TestOutput{Result: "ok", Success: true}, nil
	}

	wrapped := TracedTool("context_tool", handler)

	ctx := context.WithValue(context.Background(), key, expectedValue)
	input := TestInput{Name: "test", Value: 1}

	_, _, err := wrapped(ctx, nil, input)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if receivedValue != expectedValue {
		t.Errorf("context value = %q, want %q", receivedValue, expectedValue)
	}
}

func TestTracedTool_EmptyInput(t *testing.T) {
	type EmptyInput struct{}
	type EmptyOutput struct{}

	handler := func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, EmptyOutput, error) {
		return nil, EmptyOutput{}, nil
	}

	wrapped := TracedTool("empty_tool", handler)

	ctx := context.Background()
	input := EmptyInput{}

	_, _, err := wrapped(ctx, nil, input)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTracedTool_WithCallToolResult(t *testing.T) {
	expectedResult := &mcp.CallToolResult{
		IsError: false,
	}

	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		return expectedResult, TestOutput{Result: "ok", Success: true}, nil
	}

	wrapped := TracedTool("result_tool", handler)

	ctx := context.Background()
	input := TestInput{Name: "test", Value: 1}

	result, output, err := wrapped(ctx, nil, input)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != expectedResult {
		t.Errorf("result = %v, want %v", result, expectedResult)
	}
	if output.Result != "ok" {
		t.Errorf("output.Result = %q, want %q", output.Result, "ok")
	}
}

func TestTracedTool_MultipleCallsIndependent(t *testing.T) {
	callCount := 0
	handler := func(ctx context.Context, req *mcp.CallToolRequest, input TestInput) (*mcp.CallToolResult, TestOutput, error) {
		callCount++
		return nil, TestOutput{Result: input.Name, Success: true}, nil
	}

	wrapped := TracedTool("counter_tool", handler)

	ctx := context.Background()

	// Call multiple times
	for i := 0; i < 3; i++ {
		input := TestInput{Name: "call", Value: i}
		_, output, err := wrapped(ctx, nil, input)

		if err != nil {
			t.Errorf("call %d: unexpected error: %v", i, err)
		}
		if output.Result != "call" {
			t.Errorf("call %d: result = %q, want %q", i, output.Result, "call")
		}
	}

	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}
