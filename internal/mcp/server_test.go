package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"
)

type stdioEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func writeStdioMessage(w io.Writer, message any, newline bool) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if newline {
		payload = append(payload, '\n')
	}
	_, err = w.Write(payload)
	return err
}

func readStdioResult(ctx context.Context, lines <-chan string, wantID int) (json.RawMessage, error) {
	for {
		select {
		case line, ok := <-lines:
			if !ok {
				return nil, io.EOF
			}
			var envelope stdioEnvelope
			if err := json.Unmarshal([]byte(line), &envelope); err != nil {
				return nil, fmt.Errorf("invalid JSON on stdout %q: %w", line, err)
			}
			if envelope.JSONRPC != "2.0" {
				return nil, fmt.Errorf("unexpected JSON-RPC version in %q", line)
			}
			if envelope.ID == nil {
				continue
			}
			if *envelope.ID != wantID {
				return nil, fmt.Errorf("response ID = %d, want %d", *envelope.ID, wantID)
			}
			if len(envelope.Error) > 0 && string(envelope.Error) != "null" {
				return nil, fmt.Errorf("JSON-RPC error: %s", envelope.Error)
			}
			return envelope.Result, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func TestServerStdioProtocol(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stdinReader, stdinWriter := io.Pipe()
	stdoutReader, stdoutWriter := io.Pipe()
	defer stdinReader.Close()
	defer stdinWriter.Close()
	defer stdoutReader.Close()
	defer stdoutWriter.Close()

	srv := NewServer("stdio-test")
	serveDone := make(chan error, 1)
	go func() {
		err := srv.serveStdio(ctx, stdinReader, stdoutWriter)
		_ = stdoutWriter.Close()
		serveDone <- err
	}()

	lines := make(chan string, 8)
	scannerDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdoutReader)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scannerDone <- scanner.Err()
		close(lines)
	}()

	write := func(message any) {
		t.Helper()
		if err := writeStdioMessage(stdinWriter, message, true); err != nil {
			t.Fatalf("write MCP request: %v", err)
		}
	}
	read := func(id int) json.RawMessage {
		t.Helper()
		result, err := readStdioResult(ctx, lines, id)
		if err != nil {
			t.Fatalf("read MCP response %d: %v", id, err)
		}
		return result
	}

	write(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "melisa-test", "version": "1.0.0"},
		},
	})
	var initialized struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(read(1), &initialized); err != nil {
		t.Fatalf("decode initialize result: %v", err)
	}
	if initialized.ProtocolVersion != "2025-06-18" {
		t.Errorf("protocol version = %q", initialized.ProtocolVersion)
	}
	if initialized.ServerInfo.Name != "melisa" || initialized.ServerInfo.Version != "stdio-test" {
		t.Errorf("unexpected server info: %+v", initialized.ServerInfo)
	}

	write(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	write(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": map[string]any{}})
	var toolsResult struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(read(2), &toolsResult); err != nil {
		t.Fatalf("decode tools/list result: %v", err)
	}
	toolNames := make(map[string]bool, len(toolsResult.Tools))
	for _, tool := range toolsResult.Tools {
		toolNames[tool.Name] = true
	}
	expectedTools := map[string]bool{
		"get_health":      true,
		"collect_metrics": true,
		"explain_anomaly": true,
		"list_anomalies":  true,
	}
	if len(toolNames) != len(expectedTools) {
		t.Errorf("tool count = %d, want %d", len(toolNames), len(expectedTools))
	}
	for name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("tool %q is missing", name)
		}
	}

	if err := writeStdioMessage(stdinWriter, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": "list_anomalies", "arguments": map[string]any{}},
	}, false); err != nil {
		t.Fatalf("write final MCP request: %v", err)
	}
	if err := stdinWriter.Close(); err != nil {
		t.Fatalf("close MCP input: %v", err)
	}

	var callResult struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(read(3), &callResult); err != nil {
		t.Fatalf("decode tools/call result: %v", err)
	}
	if callResult.IsError {
		t.Fatalf("list_anomalies returned an error: %+v", callResult)
	}
	if len(callResult.Content) != 1 || callResult.Content[0].Type != "text" {
		t.Fatalf("unexpected tool content: %+v", callResult.Content)
	}
	var anomalies []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(callResult.Content[0].Text), &anomalies); err != nil {
		t.Fatalf("decode anomaly list: %v", err)
	}
	if len(anomalies) != len(anomalyExplanations) {
		t.Errorf("anomaly count = %d, want %d", len(anomalies), len(anomalyExplanations))
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve stdio after EOF: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("server did not stop after EOF")
	}
	select {
	case err := <-scannerDone:
		if err != nil {
			t.Fatalf("scan MCP output: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("MCP output scanner did not stop")
	}
	for line := range lines {
		var message any
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Errorf("invalid trailing JSON on stdout %q: %v", line, err)
		}
	}
}

func TestServerStdioContextCancellation(t *testing.T) {
	stdinReader, stdinWriter := io.Pipe()
	defer stdinReader.Close()
	defer stdinWriter.Close()

	ctx, cancel := context.WithCancel(context.Background())
	srv := NewServer("stdio-cancel-test")
	done := make(chan error, 1)
	go func() {
		done <- srv.serveStdio(ctx, stdinReader, io.Discard)
	}()

	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("serve stdio after cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after context cancellation")
	}
}
