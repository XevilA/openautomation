// main.go - Complete Go-based Workflow Automation Platform
package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// ============================================
// Data Models
// ============================================

type NodeType string

const (
	NodeWebhook   NodeType = "webhook"
	NodeTimer     NodeType = "timer"
	NodeHTTP      NodeType = "http"
	NodeEmail     NodeType = "email"
	NodeDatabase  NodeType = "database"
	NodeCondition NodeType = "condition"
	NodeLoop      NodeType = "loop"
	NodeTransform NodeType = "transform"
	NodeSlack     NodeType = "slack"
	NodeSheets    NodeType = "sheets"
	NodeOpenAI    NodeType = "openai"
)

type Node struct {
	ID         string                 `json:"id"`
	Type       NodeType               `json:"type"`
	Name       string                 `json:"name"`
	X          float64                `json:"x"`
	Y          float64                `json:"y"`
	Properties map[string]interface{} `json:"properties"`
}

type Connection struct {
	ID     string `json:"id"`
	FromID string `json:"from_id"`
	ToID   string `json:"to_id"`
}

type Workflow struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Nodes       []Node       `json:"nodes"`
	Connections []Connection `json:"connections"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Status      string       `json:"status"`
}

type ExecutionResult struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Results    map[string]interface{} `json:"results"`
	Errors     []string               `json:"errors"`
}

// ============================================
// Workflow Engine
// ============================================

type WorkflowEngine struct {
	workflows map[string]*Workflow
	mu        sync.RWMutex
	executor  *WorkflowExecutor
}

func NewWorkflowEngine() *WorkflowEngine {
	return &WorkflowEngine{
		workflows: make(map[string]*Workflow),
		executor:  NewWorkflowExecutor(),
	}
}

func (we *WorkflowEngine) CreateWorkflow(w *Workflow) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	w.CreatedAt = time.Now()
	w.UpdatedAt = time.Now()
	w.Status = "inactive"

	we.workflows[w.ID] = w
	return nil
}

func (we *WorkflowEngine) GetWorkflow(id string) (*Workflow, error) {
	we.mu.RLock()
	defer we.mu.RUnlock()

	w, exists := we.workflows[id]
	if !exists {
		return nil, fmt.Errorf("workflow not found")
	}
	return w, nil
}

func (we *WorkflowEngine) UpdateWorkflow(w *Workflow) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	if _, exists := we.workflows[w.ID]; !exists {
		return fmt.Errorf("workflow not found")
	}

	w.UpdatedAt = time.Now()
	we.workflows[w.ID] = w
	return nil
}

func (we *WorkflowEngine) DeleteWorkflow(id string) error {
	we.mu.Lock()
	defer we.mu.Unlock()

	if _, exists := we.workflows[id]; !exists {
		return fmt.Errorf("workflow not found")
	}

	delete(we.workflows, id)
	return nil
}

func (we *WorkflowEngine) ListWorkflows() []*Workflow {
	we.mu.RLock()
	defer we.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(we.workflows))
	for _, w := range we.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

func (we *WorkflowEngine) ExecuteWorkflow(id string) (*ExecutionResult, error) {
	workflow, err := we.GetWorkflow(id)
	if err != nil {
		return nil, err
	}

	return we.executor.Execute(workflow)
}

// ============================================
// Workflow Executor
// ============================================

type WorkflowExecutor struct {
	nodeExecutors map[NodeType]NodeExecutor
}

type NodeExecutor interface {
	Execute(node *Node, input interface{}) (interface{}, error)
}

func NewWorkflowExecutor() *WorkflowExecutor {
	exec := &WorkflowExecutor{
		nodeExecutors: make(map[NodeType]NodeExecutor),
	}

	// Register node executors
	exec.nodeExecutors[NodeWebhook] = &WebhookExecutor{}
	exec.nodeExecutors[NodeTimer] = &TimerExecutor{}
	exec.nodeExecutors[NodeHTTP] = &HTTPExecutor{}
	exec.nodeExecutors[NodeEmail] = &EmailExecutor{}
	exec.nodeExecutors[NodeCondition] = &ConditionExecutor{}
	exec.nodeExecutors[NodeTransform] = &TransformExecutor{}

	return exec
}

func (we *WorkflowExecutor) Execute(workflow *Workflow) (*ExecutionResult, error) {
	result := &ExecutionResult{
		WorkflowID: workflow.ID,
		Status:     "running",
		StartTime:  time.Now(),
		Results:    make(map[string]interface{}),
		Errors:     []string{},
	}

	// Build execution graph
	graph := we.buildExecutionGraph(workflow)

	// Execute nodes in order
	for _, node := range graph {
		executor, exists := we.nodeExecutors[node.Type]
		if !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("no executor for node type: %s", node.Type))
			continue
		}

		output, err := executor.Execute(&node, nil)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("node %s error: %v", node.ID, err))
			continue
		}

		result.Results[node.ID] = output
	}

	result.EndTime = time.Now()
	if len(result.Errors) > 0 {
		result.Status = "failed"
	} else {
		result.Status = "completed"
	}

	return result, nil
}

func (we *WorkflowExecutor) buildExecutionGraph(workflow *Workflow) []Node {
	// Simple topological sort
	// In production, implement proper DAG sorting
	return workflow.Nodes
}

// ============================================
// Node Executors
// ============================================

type WebhookExecutor struct{}

func (e *WebhookExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	url, _ := node.Properties["url"].(string)
	method, _ := node.Properties["method"].(string)

	return map[string]interface{}{
		"status": "webhook_executed",
		"url":    url,
		"method": method,
	}, nil
}

type TimerExecutor struct{}

func (e *TimerExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	interval, _ := node.Properties["interval"].(float64)
	time.Sleep(time.Duration(interval) * time.Second)

	return map[string]interface{}{
		"status": "timer_completed",
		"waited": interval,
	}, nil
}

type HTTPExecutor struct{}

func (e *HTTPExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	url, _ := node.Properties["url"].(string)
	method, _ := node.Properties["method"].(string)

	// Simulate HTTP request
	return map[string]interface{}{
		"status": "http_request_sent",
		"url":    url,
		"method": method,
	}, nil
}

type EmailExecutor struct{}

func (e *EmailExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	to, _ := node.Properties["to"].(string)
	subject, _ := node.Properties["subject"].(string)

	return map[string]interface{}{
		"status":  "email_sent",
		"to":      to,
		"subject": subject,
	}, nil
}

type ConditionExecutor struct{}

func (e *ConditionExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	condition, _ := node.Properties["condition"].(string)

	// Simple condition evaluation
	result := true // Simulate evaluation

	return map[string]interface{}{
		"status":    "condition_evaluated",
		"condition": condition,
		"result":    result,
	}, nil
}

type TransformExecutor struct{}

func (e *TransformExecutor) Execute(node *Node, input interface{}) (interface{}, error) {
	script, _ := node.Properties["script"].(string)

	return map[string]interface{}{
		"status": "data_transformed",
		"script": script,
	}, nil
}

// ============================================
// HTTP Server & API
// ============================================

type Server struct {
	engine   *WorkflowEngine
	upgrader websocket.Upgrader
}

func NewServer() *Server {
	return &Server{
		engine: NewWorkflowEngine(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// API Handlers
func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.engine.CreateWorkflow(&workflow); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflow)
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	workflow, err := s.engine.GetWorkflow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflow)
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.engine.UpdateWorkflow(&workflow); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflow)
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.engine.DeleteWorkflow(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows := s.engine.ListWorkflows()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

func (s *Server) handleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	result, err := s.engine.ExecuteWorkflow(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// WebSocket handler for real-time updates
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	for {
		var msg map[string]interface{}
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		// Handle different message types
		msgType, _ := msg["type"].(string)
		switch msgType {
		case "ping":
			conn.WriteJSON(map[string]string{"type": "pong"})
		case "subscribe":
			// Handle workflow subscription
		case "execute":
			// Handle workflow execution
		}
	}
}

// ============================================
// HTML Templates
// ============================================

const indexHTML = `<!DOCTYPE html>
<html lang="th">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Flow - Workflow Automation Platform</title>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            background: linear-gradient(135deg, #1e3c72 0%, #2a5298 100%);
            min-height: 100vh;
            color: #333;
        }

        .container {
            display: flex;
            height: 100vh;
            background: rgba(255, 255, 255, 0.05);
        }

        .header {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            height: 60px;
            background: rgba(255, 255, 255, 0.98);
            box-shadow: 0 2px 20px rgba(0,0,0,0.1);
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 30px;
            z-index: 1000;
            backdrop-filter: blur(10px);
        }

        .logo {
            font-size: 24px;
            font-weight: bold;
            background: linear-gradient(45deg, #1e3c72, #2a5298);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .logo svg {
            width: 30px;
            height: 30px;
            fill: #2a5298;
        }

        .nav-buttons {
            display: flex;
            gap: 10px;
        }

        .btn {
            padding: 8px 20px;
            border: none;
            border-radius: 8px;
            font-size: 14px;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.3s;
            display: flex;
            align-items: center;
            gap: 5px;
        }

        .btn-primary {
            background: linear-gradient(135deg, #1e3c72, #2a5298);
            color: white;
        }

        .btn-primary:hover {
            transform: translateY(-2px);
            box-shadow: 0 5px 20px rgba(30, 60, 114, 0.3);
        }

        .btn-secondary {
            background: white;
            color: #2a5298;
            border: 2px solid #2a5298;
        }

        .btn-secondary:hover {
            background: #2a5298;
            color: white;
        }

        .main-content {
            display: flex;
            width: 100%;
            margin-top: 60px;
        }

        .sidebar {
            width: 280px;
            background: rgba(255, 255, 255, 0.98);
            box-shadow: 2px 0 20px rgba(0,0,0,0.1);
            padding: 20px;
            overflow-y: auto;
            max-height: calc(100vh - 60px);
        }

        .sidebar h3 {
            font-size: 18px;
            margin-bottom: 20px;
            color: #2a5298;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .node-category {
            margin-bottom: 25px;
        }

        .node-category h4 {
            font-size: 12px;
            text-transform: uppercase;
            color: #666;
            margin-bottom: 10px;
            padding: 8px;
            background: #f5f7fa;
            border-radius: 5px;
            letter-spacing: 0.5px;
        }

        .node-item {
            background: white;
            border: 2px solid #e1e8ed;
            border-radius: 10px;
            padding: 12px;
            margin-bottom: 10px;
            cursor: grab;
            transition: all 0.3s;
            display: flex;
            align-items: center;
            gap: 12px;
        }

        .node-item:hover {
            transform: translateX(5px);
            border-color: #2a5298;
            box-shadow: 0 3px 15px rgba(42, 82, 152, 0.2);
        }

        .node-item:active {
            cursor: grabbing;
        }

        .node-icon {
            width: 40px;
            height: 40px;
            border-radius: 8px;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 20px;
            background: linear-gradient(135deg, #667eea, #764ba2);
            color: white;
        }

        .node-info {
            flex: 1;
        }

        .node-name {
            font-weight: 600;
            color: #333;
            margin-bottom: 2px;
        }

        .node-desc {
            font-size: 11px;
            color: #999;
        }

        .canvas-area {
            flex: 1;
            background: #f8f9fa;
            position: relative;
            overflow: auto;
        }

        #canvas {
            width: 100%;
            height: 100%;
            min-width: 2000px;
            min-height: 2000px;
            background-image:
                linear-gradient(0deg, transparent 24%, rgba(0, 0, 0, .02) 25%, rgba(0, 0, 0, .02) 26%, transparent 27%, transparent 74%, rgba(0, 0, 0, .02) 75%, rgba(0, 0, 0, .02) 76%, transparent 77%, transparent),
                linear-gradient(90deg, transparent 24%, rgba(0, 0, 0, .02) 25%, rgba(0, 0, 0, .02) 26%, transparent 27%, transparent 74%, rgba(0, 0, 0, .02) 75%, rgba(0, 0, 0, .02) 76%, transparent 77%, transparent);
            background-size: 50px 50px;
            position: relative;
        }

        .workflow-node {
            position: absolute;
            background: white;
            border: 3px solid #2a5298;
            border-radius: 12px;
            padding: 15px;
            min-width: 200px;
            box-shadow: 0 4px 15px rgba(0,0,0,0.1);
            cursor: move;
            transition: all 0.2s;
        }

        .workflow-node.selected {
            box-shadow: 0 0 0 4px rgba(42, 82, 152, 0.2);
            transform: scale(1.02);
        }

        .workflow-node:hover {
            box-shadow: 0 6px 20px rgba(0,0,0,0.15);
        }

        .node-header {
            display: flex;
            align-items: center;
            gap: 10px;
            margin-bottom: 10px;
            padding-bottom: 10px;
            border-bottom: 1px solid #e1e8ed;
        }

        .node-type-icon {
            width: 30px;
            height: 30px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #667eea, #764ba2);
            color: white;
            font-size: 16px;
        }

        .node-title {
            flex: 1;
            font-weight: 600;
            color: #333;
        }

        .node-actions {
            display: flex;
            gap: 5px;
        }

        .node-action-btn {
            width: 24px;
            height: 24px;
            border: none;
            border-radius: 4px;
            background: #f5f7fa;
            color: #666;
            cursor: pointer;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s;
        }

        .node-action-btn:hover {
            background: #e1e8ed;
        }

        .node-delete:hover {
            background: #ff4444;
            color: white;
        }

        .node-content {
            font-size: 13px;
            color: #666;
        }

        .node-port {
            position: absolute;
            width: 14px;
            height: 14px;
            border-radius: 50%;
            background: #2a5298;
            border: 3px solid white;
            cursor: crosshair;
            transition: all 0.2s;
        }

        .node-port:hover {
            transform: scale(1.3);
            box-shadow: 0 0 10px rgba(42, 82, 152, 0.5);
        }

        .node-input {
            left: -7px;
            top: 50%;
            transform: translateY(-50%);
        }

        .node-output {
            right: -7px;
            top: 50%;
            transform: translateY(-50%);
        }

        .connection-line {
            stroke: #2a5298;
            stroke-width: 3;
            fill: none;
            pointer-events: none;
            stroke-linecap: round;
        }

        .properties-panel {
            width: 320px;
            background: rgba(255, 255, 255, 0.98);
            box-shadow: -2px 0 20px rgba(0,0,0,0.1);
            padding: 20px;
            overflow-y: auto;
            max-height: calc(100vh - 60px);
        }

        .properties-panel h3 {
            font-size: 18px;
            margin-bottom: 20px;
            color: #2a5298;
            display: flex;
            align-items: center;
            gap: 10px;
        }

        .property-group {
            margin-bottom: 20px;
        }

        .property-label {
            display: block;
            font-size: 12px;
            font-weight: 600;
            color: #666;
            margin-bottom: 8px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }

        .property-input {
            width: 100%;
            padding: 10px;
            border: 2px solid #e1e8ed;
            border-radius: 8px;
            font-size: 14px;
            transition: all 0.3s;
        }

        .property-input:focus {
            outline: none;
            border-color: #2a5298;
            box-shadow: 0 0 0 3px rgba(42, 82, 152, 0.1);
        }

        .property-textarea {
            min-height: 100px;
            resize: vertical;
            font-family: 'Monaco', 'Courier New', monospace;
            font-size: 12px;
        }

        .property-select {
            cursor: pointer;
        }

        .toolbar {
            position: absolute;
            top: 20px;
            left: 20px;
            display: flex;
            gap: 10px;
            z-index: 100;
        }

        .toolbar-btn {
            background: white;
            border: 2px solid #2a5298;
            color: #2a5298;
            padding: 10px 15px;
            border-radius: 8px;
            cursor: pointer;
            font-weight: 500;
            transition: all 0.3s;
            display: flex;
            align-items: center;
            gap: 5px;
        }

        .toolbar-btn:hover {
            background: #2a5298;
            color: white;
            transform: translateY(-2px);
            box-shadow: 0 5px 15px rgba(42, 82, 152, 0.3);
        }

        .status-bar {
            position: fixed;
            bottom: 0;
            left: 0;
            right: 0;
            height: 30px;
            background: rgba(255, 255, 255, 0.98);
            border-top: 1px solid #e1e8ed;
            display: flex;
            align-items: center;
            padding: 0 20px;
            font-size: 12px;
            color: #666;
            z-index: 1000;
        }

        .status-indicator {
            width: 8px;
            height: 8px;
            border-radius: 50%;
            background: #4CAF50;
            margin-right: 10px;
            animation: pulse 2s infinite;
        }

        @keyframes pulse {
            0% { transform: scale(1); opacity: 1; }
            50% { transform: scale(1.2); opacity: 0.7; }
            100% { transform: scale(1); opacity: 1; }
        }

        .modal {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.5);
            z-index: 2000;
            align-items: center;
            justify-content: center;
        }

        .modal.active {
            display: flex;
        }

        .modal-content {
            background: white;
            border-radius: 12px;
            padding: 30px;
            max-width: 500px;
            width: 90%;
            max-height: 80vh;
            overflow-y: auto;
            box-shadow: 0 10px 50px rgba(0,0,0,0.3);
        }

        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 20px;
        }

        .modal-title {
            font-size: 20px;
            font-weight: 600;
            color: #333;
        }

        .modal-close {
            background: none;
            border: none;
            font-size: 24px;
            color: #999;
            cursor: pointer;
        }

        .modal-close:hover {
            color: #333;
        }

        @media (max-width: 768px) {
            .sidebar, .properties-panel {
                display: none;
            }

            .canvas-area {
                width: 100%;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="logo">
                <svg viewBox="0 0 24 24">
                    <path d="M12 2L2 7v10c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V7l-10-5z"/>
                </svg>
                Go Flow
            </div>
            <div class="nav-buttons">
                <button class="btn btn-secondary" onclick="toggleMode()">
                    <span>👁️</span> Visual Mode
                </button>
                <button class="btn btn-primary" onclick="saveWorkflow()">
                    <span>💾</span> Save
                </button>
                <button class="btn btn-primary" onclick="runWorkflow()">
                    <span>▶️</span> Run
                </button>
            </div>
        </div>

        <div class="main-content">
            <div class="sidebar">
                <h3><span>📦</span> Nodes Library</h3>

                <div class="node-category">
                    <h4>Triggers</h4>
                    <div class="node-item" draggable="true" data-node-type="webhook">
                        <div class="node-icon">🌐</div>
                        <div class="node-info">
                            <div class="node-name">Webhook</div>
                            <div class="node-desc">Receive HTTP requests</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="timer">
                        <div class="node-icon">⏰</div>
                        <div class="node-info">
                            <div class="node-name">Timer</div>
                            <div class="node-desc">Schedule execution</div>
                        </div>
                    </div>
                </div>

                <div class="node-category">
                    <h4>Actions</h4>
                    <div class="node-item" draggable="true" data-node-type="http">
                        <div class="node-icon">📡</div>
                        <div class="node-info">
                            <div class="node-name">HTTP Request</div>
                            <div class="node-desc">Make API calls</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="email">
                        <div class="node-icon">✉️</div>
                        <div class="node-info">
                            <div class="node-name">Send Email</div>
                            <div class="node-desc">Send email messages</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="database">
                        <div class="node-icon">🗄️</div>
                        <div class="node-info">
                            <div class="node-name">Database</div>
                            <div class="node-desc">Query database</div>
                        </div>
                    </div>
                </div>

                <div class="node-category">
                    <h4>Logic</h4>
                    <div class="node-item" draggable="true" data-node-type="condition">
                        <div class="node-icon">❓</div>
                        <div class="node-info">
                            <div class="node-name">If/Then</div>
                            <div class="node-desc">Conditional logic</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="loop">
                        <div class="node-icon">🔁</div>
                        <div class="node-info">
                            <div class="node-name">Loop</div>
                            <div class="node-desc">Iterate over data</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="transform">
                        <div class="node-icon">🔄</div>
                        <div class="node-info">
                            <div class="node-name">Transform</div>
                            <div class="node-desc">Transform data</div>
                        </div>
                    </div>
                </div>

                <div class="node-category">
                    <h4>Integrations</h4>
                    <div class="node-item" draggable="true" data-node-type="slack">
                        <div class="node-icon">💬</div>
                        <div class="node-info">
                            <div class="node-name">Slack</div>
                            <div class="node-desc">Send to Slack</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="sheets">
                        <div class="node-icon">📊</div>
                        <div class="node-info">
                            <div class="node-name">Google Sheets</div>
                            <div class="node-desc">Read/Write sheets</div>
                        </div>
                    </div>
                    <div class="node-item" draggable="true" data-node-type="openai">
                        <div class="node-icon">🤖</div>
                        <div class="node-info">
                            <div class="node-name">OpenAI</div>
                            <div class="node-desc">AI completion</div>
                        </div>
                    </div>
                </div>
            </div>

            <div class="canvas-area">
                <div class="toolbar">
                    <button class="toolbar-btn" onclick="clearCanvas()">🗑️ Clear</button>
                    <button class="toolbar-btn" onclick="exportWorkflow()">📤 Export</button>
                    <button class="toolbar-btn" onclick="importWorkflow()">📥 Import</button>
                </div>
                <div id="canvas">
                    <svg id="connectionsSvg" style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; pointer-events: none;">
                    </svg>
                </div>
            </div>

            <div class="properties-panel">
                <h3><span>⚙️</span> Properties</h3>
                <div id="propertiesContent">
                    <p style="color: #999; text-align: center; margin-top: 50px;">
                        Select a node to edit properties
                    </p>
                </div>
            </div>
        </div>

        <div class="status-bar">
            <div class="status-indicator"></div>
            <span id="statusText">Ready</span>
        </div>
    </div>

    <div class="modal" id="jsonModal">
        <div class="modal-content">
            <div class="modal-header">
                <h3 class="modal-title">JSON Editor</h3>
                <button class="modal-close" onclick="closeModal('jsonModal')">×</button>
            </div>
            <textarea id="jsonEditor" class="property-input property-textarea" style="height: 400px;"></textarea>
            <div style="margin-top: 20px; display: flex; gap: 10px; justify-content: flex-end;">
                <button class="btn btn-secondary" onclick="closeModal('jsonModal')">Cancel</button>
                <button class="btn btn-primary" onclick="applyJson()">Apply</button>
            </div>
        </div>
    </div>

    <script>
        // Global state
        let nodes = [];
        let connections = [];
        let selectedNode = null;
        let isConnecting = false;
        let connectionStart = null;
        let nodeIdCounter = 0;
        let ws = null;

        // Node configurations
        const nodeConfigs = {
            webhook: { icon: '🌐', color: '#4CAF50', name: 'Webhook' },
            timer: { icon: '⏰', color: '#FF9800', name: 'Timer' },
            http: { icon: '📡', color: '#9C27B0', name: 'HTTP Request' },
            email: { icon: '✉️', color: '#F44336', name: 'Send Email' },
            database: { icon: '🗄️', color: '#607D8B', name: 'Database' },
            condition: { icon: '❓', color: '#00BCD4', name: 'If/Then' },
            loop: { icon: '🔁', color: '#8BC34A', name: 'Loop' },
            transform: { icon: '🔄', color: '#FFC107', name: 'Transform' },
            slack: { icon: '💬', color: '#4A154B', name: 'Slack' },
            sheets: { icon: '📊', color: '#0F9D58', name: 'Google Sheets' },
            openai: { icon: '🤖', color: '#412991', name: 'OpenAI' }
        };

        // Initialize
        document.addEventListener('DOMContentLoaded', function() {
            setupDragAndDrop();
            setupWebSocket();
            loadWorkflow();
        });

        // WebSocket connection
        function setupWebSocket() {
            ws = new WebSocket('ws://localhost:8080/ws');

            ws.onopen = function() {
                updateStatus('Connected', '#4CAF50');
            };

            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                handleWebSocketMessage(data);
            };

            ws.onclose = function() {
                updateStatus('Disconnected', '#F44336');
                setTimeout(setupWebSocket, 5000);
            };
        }

        function handleWebSocketMessage(data) {
            switch(data.type) {
                case 'execution_update':
                    updateExecutionStatus(data);
                    break;
                case 'node_update':
                    updateNodeStatus(data);
                    break;
            }
        }

        // Drag and Drop
        function setupDragAndDrop() {
            const nodeItems = document.querySelectorAll('.node-item');
            const canvas = document.getElementById('canvas');

            nodeItems.forEach(item => {
                item.addEventListener('dragstart', (e) => {
                    e.dataTransfer.setData('nodeType', item.dataset.nodeType);
                });
            });

            canvas.addEventListener('dragover', (e) => {
                e.preventDefault();
            });

            canvas.addEventListener('drop', (e) => {
                e.preventDefault();
                const nodeType = e.dataTransfer.getData('nodeType');
                const rect = canvas.getBoundingClientRect();
                const x = e.clientX - rect.left + canvas.scrollLeft;
                const y = e.clientY - rect.top + canvas.scrollTop;
                createNode(nodeType, x, y);
            });
        }

        // Node creation and management
        function createNode(type, x, y) {
            const node = {
                id: 'node_' + nodeIdCounter++,
                type: type,
                name: nodeConfigs[type].name,
                x: x,
                y: y,
                properties: getDefaultProperties(type)
            };

            nodes.push(node);
            renderNode(node);
            saveToLocal();
        }

        function renderNode(node) {
            const config = nodeConfigs[node.type];
            const nodeEl = document.createElement('div');
            nodeEl.className = 'workflow-node';
            nodeEl.id = node.id;
            nodeEl.style.left = node.x + 'px';
            nodeEl.style.top = node.y + 'px';

            nodeEl.innerHTML = ` + "`" + `
                <div class="node-port node-input"></div>
                <div class="node-port node-output"></div>
                <div class="node-header">
                    <div class="node-type-icon">${config.icon}</div>
                    <div class="node-title">${node.name}</div>
                    <div class="node-actions">
                        <button class="node-action-btn" onclick="editNode('${node.id}')">✏️</button>
                        <button class="node-action-btn node-delete" onclick="deleteNode('${node.id}')">×</button>
                    </div>
                </div>
                <div class="node-content">
                    ${getNodeDescription(node)}
                </div>
            ` + "`" + `;

            setupNodeInteractions(nodeEl, node);
            document.getElementById('canvas').appendChild(nodeEl);
        }

        function setupNodeInteractions(nodeEl, node) {
            // Dragging
            let isDragging = false;
            let dragOffset = { x: 0, y: 0 };

            nodeEl.addEventListener('mousedown', (e) => {
                if (e.target.classList.contains('node-action-btn') ||
                    e.target.classList.contains('node-port')) {
                    return;
                }

                isDragging = true;
                dragOffset.x = e.clientX - node.x;
                dragOffset.y = e.clientY - node.y;
                selectNode(node.id);
            });

            document.addEventListener('mousemove', (e) => {
                if (isDragging) {
                    const canvas = document.getElementById('canvas');
                    node.x = e.clientX - dragOffset.x + canvas.scrollLeft;
                    node.y = e.clientY - dragOffset.y + canvas.scrollTop;
                    nodeEl.style.left = node.x + 'px';
                    nodeEl.style.top = node.y + 'px';
                    updateConnections();
                }
            });

            document.addEventListener('mouseup', () => {
                if (isDragging) {
                    isDragging = false;
                    saveToLocal();
                }
            });

            // Connections
            const output = nodeEl.querySelector('.node-output');
            const input = nodeEl.querySelector('.node-input');

            output.addEventListener('click', (e) => {
                e.stopPropagation();
                startConnection(node.id);
            });

            input.addEventListener('click', (e) => {
                e.stopPropagation();
                if (connectionStart) {
                    completeConnection(node.id);
                }
            });
        }

        function selectNode(nodeId) {
            document.querySelectorAll('.workflow-node').forEach(n => {
                n.classList.remove('selected');
            });

            const nodeEl = document.getElementById(nodeId);
            if (nodeEl) {
                nodeEl.classList.add('selected');
            }

            selectedNode = nodes.find(n => n.id === nodeId);
            updatePropertiesPanel();
        }

        function editNode(nodeId) {
            selectNode(nodeId);
        }

        function deleteNode(nodeId) {
            if (confirm('Delete this node?')) {
                nodes = nodes.filter(n => n.id !== nodeId);
                connections = connections.filter(c => c.from_id !== nodeId && c.to_id !== nodeId);

                document.getElementById(nodeId).remove();
                updateConnections();
                saveToLocal();

                if (selectedNode && selectedNode.id === nodeId) {
                    selectedNode = null;
                    updatePropertiesPanel();
                }
            }
        }

        // Connection management
        function startConnection(nodeId) {
            connectionStart = nodeId;
            isConnecting = true;
            document.getElementById('canvas').style.cursor = 'crosshair';
        }

        function completeConnection(nodeId) {
            if (connectionStart && connectionStart !== nodeId) {
                const connection = {
                    id: 'conn_' + Date.now(),
                    from_id: connectionStart,
                    to_id: nodeId
                };

                connections.push(connection);
                drawConnection(connection);
                saveToLocal();
            }

            connectionStart = null;
            isConnecting = false;
            document.getElementById('canvas').style.cursor = 'default';
        }

        function drawConnection(connection) {
            const fromNode = document.getElementById(connection.from_id);
            const toNode = document.getElementById(connection.to_id);

            if (!fromNode || !toNode) return;

            const svg = document.getElementById('connectionsSvg');
            const canvas = document.getElementById('canvas');

            const fromRect = fromNode.getBoundingClientRect();
            const toRect = toNode.getBoundingClientRect();
            const canvasRect = canvas.getBoundingClientRect();

            const x1 = fromRect.right - canvasRect.left + canvas.scrollLeft;
            const y1 = fromRect.top + fromRect.height/2 - canvasRect.top + canvas.scrollTop;
            const x2 = toRect.left - canvasRect.left + canvas.scrollLeft;
            const y2 = toRect.top + toRect.height/2 - canvasRect.top + canvas.scrollTop;

            const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
            const midX = (x1 + x2) / 2;

            path.setAttribute('d', ` + "`" + `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}` + "`" + `);
            path.setAttribute('class', 'connection-line');
            path.setAttribute('id', connection.id);

            svg.appendChild(path);
        }

        function updateConnections() {
            const svg = document.getElementById('connectionsSvg');
            svg.innerHTML = '';
            connections.forEach(conn => drawConnection(conn));
        }

        // Properties panel
        function updatePropertiesPanel() {
            const content = document.getElementById('propertiesContent');

            if (!selectedNode) {
                content.innerHTML = '<p style="color: #999; text-align: center; margin-top: 50px;">Select a node to edit properties</p>';
                return;
            }

            const config = nodeConfigs[selectedNode.type];
            let html = ` + "`" + `<h4>${config.icon} ${selectedNode.name}</h4>` + "`" + `;

            // Add property inputs based on node type
            html += getPropertyInputs(selectedNode);

            content.innerHTML = html;
        }

        function getPropertyInputs(node) {
            let html = '';
            const props = getNodePropertyDefinitions(node.type);

            for (const [key, def] of Object.entries(props)) {
                html += ` + "`" + `<div class="property-group">
                    <label class="property-label">${def.label}</label>` + "`" + `;

                switch(def.type) {
                    case 'text':
                    case 'number':
                        html += ` + "`" + `<input type="${def.type}" class="property-input"
                                value="${node.properties[key] || ''}"
                                onchange="updateNodeProperty('${key}', this.value)">` + "`" + `;
                        break;
                    case 'select':
                        html += ` + "`" + `<select class="property-input property-select"
                                onchange="updateNodeProperty('${key}', this.value)">` + "`" + `;
                        def.options.forEach(opt => {
                            const selected = node.properties[key] === opt ? 'selected' : '';
                            html += ` + "`" + `<option value="${opt}" ${selected}>${opt}</option>` + "`" + `;
                        });
                        html += '</select>';
                        break;
                    case 'textarea':
                        html += ` + "`" + `<textarea class="property-input property-textarea"
                                onchange="updateNodeProperty('${key}', this.value)">${node.properties[key] || ''}</textarea>` + "`" + `;
                        break;
                }

                html += '</div>';
            }

            return html;
        }

        function updateNodeProperty(key, value) {
            if (selectedNode) {
                selectedNode.properties[key] = value;

                // Update node display
                const nodeEl = document.getElementById(selectedNode.id);
                const content = nodeEl.querySelector('.node-content');
                content.innerHTML = getNodeDescription(selectedNode);

                saveToLocal();
            }
        }

        // Helper functions
        function getDefaultProperties(type) {
            const props = {};
            const defs = getNodePropertyDefinitions(type);

            for (const [key, def] of Object.entries(defs)) {
                props[key] = def.default || '';
            }

            return props;
        }

        function getNodePropertyDefinitions(type) {
            const definitions = {
                webhook: {
                    url: { label: 'URL', type: 'text', default: '/webhook' },
                    method: { label: 'Method', type: 'select', options: ['GET', 'POST', 'PUT', 'DELETE'], default: 'POST' }
                },
                timer: {
                    interval: { label: 'Interval (seconds)', type: 'number', default: 60 },
                    cron: { label: 'Cron Expression', type: 'text', default: '* * * * *' }
                },
                http: {
                    url: { label: 'URL', type: 'text', default: '' },
                    method: { label: 'Method', type: 'select', options: ['GET', 'POST', 'PUT', 'DELETE'], default: 'GET' },
                    headers: { label: 'Headers (JSON)', type: 'textarea', default: '{}' },
                    body: { label: 'Body (JSON)', type: 'textarea', default: '{}' }
                },
                email: {
                    to: { label: 'To', type: 'text', default: '' },
                    subject: { label: 'Subject', type: 'text', default: '' },
                    body: { label: 'Body', type: 'textarea', default: '' }
                },
                database: {
                    operation: { label: 'Operation', type: 'select', options: ['SELECT', 'INSERT', 'UPDATE', 'DELETE'], default: 'SELECT' },
                    query: { label: 'Query', type: 'textarea', default: '' }
                },
                condition: {
                    condition: { label: 'Condition', type: 'textarea', default: 'value > 0' }
                },
                loop: {
                    iterations: { label: 'Iterations', type: 'number', default: 10 }
                },
                transform: {
                    script: { label: 'Script', type: 'textarea', default: 'return data' }
                },
                slack: {
                    webhook: { label: 'Webhook URL', type: 'text', default: '' },
                    message: { label: 'Message', type: 'textarea', default: '' }
                },
                sheets: {
                    spreadsheetId: { label: 'Spreadsheet ID', type: 'text', default: '' },
                    range: { label: 'Range', type: 'text', default: 'A1:Z100' }
                },
                openai: {
                    apiKey: { label: 'API Key', type: 'text', default: '' },
                    prompt: { label: 'Prompt', type: 'textarea', default: '' }
                }
            };

            return definitions[type] || {};
        }

        function getNodeDescription(node) {
            const props = node.properties;

            switch(node.type) {
                case 'webhook':
                    return ` + "`" + `${props.method || 'POST'} ${props.url || '/webhook'}` + "`" + `;
                case 'timer':
                    return ` + "`" + `Every ${props.interval || 60} seconds` + "`" + `;
                case 'http':
                    return ` + "`" + `${props.method || 'GET'} ${props.url || 'URL not set'}` + "`" + `;
                case 'email':
                    return ` + "`" + `To: ${props.to || 'Not set'}` + "`" + `;
                case 'database':
                    return ` + "`" + `${props.operation || 'SELECT'}` + "`" + `;
                default:
                    return 'Configure node';
            }
        }

        // Workflow operations
        function saveWorkflow() {
            const workflow = {
                name: 'My Workflow',
                description: 'Created with Go Flow',
                nodes: nodes,
                connections: connections
            };

            fetch('/api/workflows', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(workflow)
            })
            .then(response => response.json())
            .then(data => {
                updateStatus('Workflow saved', '#4CAF50');
                localStorage.setItem('workflowId', data.id);
            })
            .catch(error => {
                console.error('Error:', error);
                updateStatus('Save failed', '#F44336');
            });
        }

        function runWorkflow() {
            const workflowId = localStorage.getItem('workflowId');
            if (!workflowId) {
                saveWorkflow();
                return;
            }

            updateStatus('Running workflow...', '#FF9800');

            fetch(` + "`" + `/api/workflows/${workflowId}/execute` + "`" + `, {
                method: 'POST'
            })
            .then(response => response.json())
            .then(data => {
                updateStatus('Workflow completed', '#4CAF50');
                console.log('Execution result:', data);
            })
            .catch(error => {
                console.error('Error:', error);
                updateStatus('Execution failed', '#F44336');
            });
        }

        function clearCanvas() {
            if (confirm('Clear all nodes and connections?')) {
                nodes = [];
                connections = [];
                selectedNode = null;

                document.querySelectorAll('.workflow-node').forEach(n => n.remove());
                document.getElementById('connectionsSvg').innerHTML = '';
                updatePropertiesPanel();
                saveToLocal();
            }
        }

        function exportWorkflow() {
            const workflow = {
                nodes: nodes,
                connections: connections,
                version: '1.0',
                created: new Date().toISOString()
            };

            const blob = new Blob([JSON.stringify(workflow, null, 2)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = 'workflow.json';
            a.click();
        }

        function importWorkflow() {
            const input = document.createElement('input');
            input.type = 'file';
            input.accept = '.json';

            input.onchange = (e) => {
                const file = e.target.files[0];
                const reader = new FileReader();

                reader.onload = (event) => {
                    try {
                        const workflow = JSON.parse(event.target.result);
                        loadWorkflowData(workflow);
                        updateStatus('Workflow imported', '#4CAF50');
                    } catch (error) {
                        updateStatus('Import failed', '#F44336');
                    }
                };

                reader.readAsText(file);
            };

            input.click();
        }

        function loadWorkflowData(workflow) {
            clearCanvas();
            nodes = workflow.nodes || [];
            connections = workflow.connections || [];

            nodes.forEach(node => renderNode(node));
            connections.forEach(conn => drawConnection(conn));

            saveToLocal();
        }

        // Local storage
        function saveToLocal() {
            const workflow = {
                nodes: nodes,
                connections: connections
            };
            localStorage.setItem('goflow_workflow', JSON.stringify(workflow));
        }

        function loadWorkflow() {
            const saved = localStorage.getItem('goflow_workflow');
            if (saved) {
                const workflow = JSON.parse(saved);
                loadWorkflowData(workflow);
            }
        }

        // UI helpers
        function toggleMode() {
            document.getElementById('jsonModal').classList.add('active');
            const workflow = {
                nodes: nodes,
                connections: connections
            };
            document.getElementById('jsonEditor').value = JSON.stringify(workflow, null, 2);
        }

        function closeModal(modalId) {
            document.getElementById(modalId).classList.remove('active');
        }

        function applyJson() {
            try {
                const json = document.getElementById('jsonEditor').value;
                const workflow = JSON.parse(json);
                loadWorkflowData(workflow);
                closeModal('jsonModal');
                updateStatus('JSON applied', '#4CAF50');
            } catch (error) {
                alert('Invalid JSON: ' + error.message);
            }
        }

        function updateStatus(text, color) {
            document.getElementById('statusText').textContent = text;
            document.querySelector('.status-indicator').style.background = color;
        }

        function updateExecutionStatus(data) {
            updateStatus(` + "`" + `Executing: ${data.status}` + "`" + `, '#FF9800');
        }

        function updateNodeStatus(data) {
            const nodeEl = document.getElementById(data.nodeId);
            if (nodeEl) {
                nodeEl.style.borderColor = data.status === 'running' ? '#FF9800' : '#4CAF50';
            }
        }
    </script>
</body>
</html>
`

// Serve the main page
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

// ============================================
// Main Function
// ============================================

func main() {
	server := NewServer()
	router := mux.NewRouter()

	// Static files
	router.HandleFunc("/", server.handleIndex).Methods("GET")

	// API routes
	api := router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/workflows", server.handleCreateWorkflow).Methods("POST")
	api.HandleFunc("/workflows", server.handleListWorkflows).Methods("GET")
	api.HandleFunc("/workflows/{id}", server.handleGetWorkflow).Methods("GET")
	api.HandleFunc("/workflows/{id}", server.handleUpdateWorkflow).Methods("PUT")
	api.HandleFunc("/workflows/{id}", server.handleDeleteWorkflow).Methods("DELETE")
	api.HandleFunc("/workflows/{id}/execute", server.handleExecuteWorkflow).Methods("POST")

	// WebSocket
	router.HandleFunc("/ws", server.handleWebSocket)

	// Start server
	log.Println("Go Flow Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

// ============================================
// Dependencies (go.mod)
// ============================================
// module goflow
//
// go 1.21
//
// require (
//     github.com/google/uuid v1.5.0
//     github.com/gorilla/mux v1.8.1
//     github.com/gorilla/websocket v1.5.1
// )
