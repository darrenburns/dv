package main

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func newMCPServer(store ReviewStore, repo string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "aside", Version: "0.1.0"}, nil)
	operations := map[string]string{
		"open":     "Open a local working-tree, branch, or PR review. Captures an immutable diff snapshot; does not change the checkout. Return its ID to the user to open with aside --review ID.",
		"list":     "List saved reviews for this repository.",
		"read":     "Read a review's diff snapshots and inline threads. Use exact snapshot ID, path, old/new side, and line numbers from this data when publishing.",
		"publish":  "Publish 1–100 inline findings or explanations atomically. Each annotation needs a stable key for safe retries and an exact snapshot/path/side/start/end. Only writes local review data.",
		"feedback": "Read submitted user feedback and agent threads since a revision cursor. Drafts are excluded. Does not wake an idle agent.",
		"reply":    "Reply to a review thread; preserve its original code anchor.",
		"status":   "Mark a thread open or addressed. Reserve resolved for the user after verification.",
	}
	for operation, description := range operations {
		mcp.AddTool(server, &mcp.Tool{Name: "aside_" + operation, Description: description}, func(ctx context.Context, request *mcp.CallToolRequest, input ReviewRequest) (*mcp.CallToolResult, any, error) {
			if operation == "status" && input.Status == "resolved" {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "Agents may mark addressed; users resolve threads in Aside."}}}, nil, nil
			}
			result, err := reviewOperation(store, repo, operation, input)
			if err != nil {
				return nil, nil, err
			}
			data, err := json.Marshal(result)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, nil
		})
	}
	return server
}
func serveMCP(ctx context.Context, store ReviewStore, repo string) error {
	return newMCPServer(store, repo).Run(ctx, &mcp.StdioTransport{})
}
