// Distributed MemOS - MCP Server Implementation
// Implements Model Context Protocol to expose MemOS as an LLM tool
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/mohi1038/memos/internal/api"
)

// MCPServer implements the Model Context Protocol for MemOS
// This allows any LLM to use MemOS as a tool/resource
type MCPServer struct {
	handler *api.MemoryHandler
	addr    string
	port    int
}

// MCPCapabilities describes what this MCP server can do
type MCPCapabilities struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolDefinition describes an MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPRequest represents an incoming MCP request
type MCPRequest struct {
	JSONRPCVersion string                 `json:"jsonrpc"`
	ID             int64                  `json:"id"`
	Method         string                 `json:"method"`
	Params         map[string]interface{} `json:"params"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPCVersion string      `json:"jsonrpc"`
	ID             int64       `json:"id"`
	Result         interface{} `json:"result,omitempty"`
	Error          *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// StoreToolInput for MCP store request
type StoreToolInput struct {
	TenantID   string  `json:"tenantId"`
	AgentID    string  `json:"agentId"`
	Content    string  `json:"content"`
	Type       string  `json:"type,omitempty"`
	Importance float32 `json:"importance,omitempty"`
}

// RetrieveToolInput for MCP retrieve request
type RetrieveToolInput struct {
	TenantID   string  `json:"tenantId"`
	AgentID    string  `json:"agentId"`
	Query      string  `json:"query"`
	Limit      int32   `json:"limit,omitempty"`
	Threshold  float32 `json:"similarityThreshold,omitempty"`
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(handler *api.MemoryHandler, addr string, port int) *MCPServer {
	return &MCPServer{
		handler: handler,
		addr:    addr,
		port:    port,
	}
}

// Start runs the MCP server
func (s *MCPServer) Start() error {
	http.HandleFunc("/", s.handleRequest)
	http.HandleFunc("/capabilities", s.handleCapabilities)
	http.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf("%s:%d", s.addr, s.port)
	log.Printf("MCP Server listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleCapabilities returns the MCP server capabilities
func (s *MCPServer) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	caps := MCPCapabilities{
		Tools: []ToolDefinition{
			{
				Name:        "store_memory",
				Description: "Store a memory for an agent in MemOS",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tenantId":    map[string]string{"type": "string", "description": "Tenant ID"},
						"agentId":     map[string]string{"type": "string", "description": "Agent ID"},
						"content":     map[string]string{"type": "string", "description": "Memory content"},
						"type":        map[string]string{"type": "string", "description": "Memory type (episodic, semantic, etc)"},
						"importance":  map[string]string{"type": "number", "description": "Importance score 0-1"},
					},
					"required": []string{"tenantId", "agentId", "content"},
				},
			},
			{
				Name:        "retrieve_memory",
				Description: "Search for memories related to a query",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tenantId":            map[string]string{"type": "string", "description": "Tenant ID"},
						"agentId":             map[string]string{"type": "string", "description": "Agent ID"},
						"query":               map[string]string{"type": "string", "description": "Search query"},
						"limit":               map[string]string{"type": "integer", "description": "Max results (default 10)"},
						"similarityThreshold": map[string]string{"type": "number", "description": "Min similarity score 0-1"},
					},
					"required": []string{"tenantId", "agentId", "query"},
				},
			},
		},
	}

	json.NewEncoder(w).Encode(caps)
}

// handleHealth returns server health status
func (s *MCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// handleRequest processes incoming MCP requests
func (s *MCPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MCPResponse{
			JSONRPCVersion: "2.0",
			ID:             req.ID,
			Error: &MCPError{
				Code:    -32700,
				Message: "Parse error: " + err.Error(),
			},
		})
		return
	}

	// Route to appropriate tool handler
	var result interface{}
	var err error

	switch req.Method {
	case "call_tool":
		toolName, ok := req.Params["name"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(MCPResponse{
				JSONRPCVersion: "2.0",
				ID:             req.ID,
				Error: &MCPError{Code: -32602, Message: "Invalid params: missing tool name"},
			})
			return
		}

		toolInput, ok := req.Params["arguments"].(map[string]interface{})
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(MCPResponse{
				JSONRPCVersion: "2.0",
				ID:             req.ID,
				Error: &MCPError{Code: -32602, Message: "Invalid params: missing arguments"},
			})
			return
		}

		switch toolName {
		case "store_memory":
			result, err = s.handleStoreTool(r.Context(), toolInput)
		case "retrieve_memory":
			result, err = s.handleRetrieveTool(r.Context(), toolInput)
		default:
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(MCPResponse{
				JSONRPCVersion: "2.0",
				ID:             req.ID,
				Error: &MCPError{Code: -32601, Message: "Unknown tool: " + toolName},
			})
			return
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(MCPResponse{
			JSONRPCVersion: "2.0",
			ID:             req.ID,
			Error: &MCPError{Code: -32601, Message: "Unknown method: " + req.Method},
		})
		return
	}

	// Send response
	resp := MCPResponse{
		JSONRPCVersion: "2.0",
		ID:             req.ID,
	}

	if err != nil {
		resp.Error = &MCPError{Code: -32603, Message: err.Error()}
	} else {
		resp.Result = result
	}

	json.NewEncoder(w).Encode(resp)
}

// handleStoreTool handles store_memory tool calls
func (s *MCPServer) handleStoreTool(_ context.Context, input map[string]interface{}) (interface{}, error) {
	// Parse input
	tenantID, _ := input["tenantId"].(string)
	agentID, _ := input["agentId"].(string)
	content, _ := input["content"].(string)
	memType, _ := input["type"].(string)
	importance, _ := input["importance"].(float64)

	if tenantID == "" || agentID == "" || content == "" {
		return nil, fmt.Errorf("missing required fields: tenantId, agentId, content")
	}

	if memType == "" {
		memType = "MEMORY_TYPE_EPISODIC"
	}
	if importance == 0 {
		importance = 0.5
	}

	log.Printf("[MCP] Store: %s -> agent=%s, content_len=%d, type=%s, importance=%.1f",
		tenantID, agentID, len(content), memType, importance)

	// This would call the actual gRPC handler
	return map[string]interface{}{
		"success":   true,
		"memoryId": uuid.New().String(),
		"message":  "Memory stored successfully",
	}, nil
}

// handleRetrieveTool handles retrieve_memory tool calls
func (s *MCPServer) handleRetrieveTool(_ context.Context, input map[string]interface{}) (interface{}, error) {
	// Parse input
	tenantID, _ := input["tenantId"].(string)
	agentID, _ := input["agentId"].(string)
	query, _ := input["query"].(string)
	limit, _ := input["limit"].(float64)
	threshold, _ := input["similarityThreshold"].(float64)

	if tenantID == "" || agentID == "" || query == "" {
		return nil, fmt.Errorf("missing required fields: tenantId, agentId, query")
	}

	if limit == 0 {
		limit = 10
	}
	if threshold == 0 {
		threshold = 0.5
	}

	log.Printf("[MCP] Retrieve: %s -> agent=%s, query='%s', limit=%.0f, threshold=%.2f",
		tenantID, agentID, query, limit, threshold)

	// This would call the actual gRPC handler
	return map[string]interface{}{
		"success":   true,
		"memories": []interface{}{},
		"message":  fmt.Sprintf("Retrieved 0 memories for query: %s", query),
	}, nil
}

// MCPServerMain starts the MCP server
func MCPServerMain() {
	// Initialize storage (placeholder)
	log.Println("MCP Server initialization...")

	// Get configuration from environment
	addr := os.Getenv("MCP_ADDR")
	if addr == "" {
		addr = "localhost"
	}

	portStr := os.Getenv("MCP_PORT")
	port := 8080
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	// TODO: Initialize actual MemOS handler with database connections
	var handler *api.MemoryHandler
	// handler = createMemoryHandler()

	server := NewMCPServer(handler, addr, port)
	if err := server.Start(); err != nil {
		log.Fatalf("MCP Server failed to start: %v", err)
	}
}
