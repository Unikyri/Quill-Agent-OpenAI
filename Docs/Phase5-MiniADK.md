# Phase 5: The Ultimate MemoryAgent (Mini ADK + PRD Fulfillment)

## Objective
Transform Quill's static, linear "fake AI" pipeline into a true **MemoryAgent** by building a custom, lightweight ReAct (Reasoning and Acting) loop in Go. Additionally, this phase will implement the missing critical features from the PRD/SRS (Document Ingestion and Intelligent Timeline) to guarantee maximum points in "Technical Depth" for the Hackathon.

---

## 1. Core LLM Orchestration (`backend/internal/services/qwen_service.go`)

Currently, `QwenService` only supports basic chat completion. We must upgrade it to support **Function Calling (Tools)** and an **Agent Loop**.

### 1.1 Struct Modifications
Add the necessary fields to support the OpenAI tool calling specification:

```go
type QwenRequest struct {
    Model       string        `json:"model"`
    Messages    []QwenMessage `json:"messages"`
    Tools       []QwenTool    `json:"tools,omitempty"`
    ToolChoice  interface{}   `json:"tool_choice,omitempty"` // e.g., "auto"
}

type QwenTool struct {
    Type     string           `json:"type"` // Must be "function"
    Function QwenToolFunction `json:"function"`
}

type QwenToolFunction struct {
    Name        string                 `json:"name"`
    Description string                 `json:"description"`
    Parameters  map[string]interface{} `json:"parameters"` // JSON Schema of arguments
}

type QwenMessage struct {
    Role       string         `json:"role"`
    Content    string         `json:"content,omitempty"`
    Name       string         `json:"name,omitempty"`
    ToolCalls  []QwenToolCall `json:"tool_calls,omitempty"`
    ToolCallID string         `json:"tool_call_id,omitempty"`
}

type QwenToolCall struct {
    ID       string `json:"id"`
    Type     string `json:"type"` // "function"
    Function struct {
        Name      string `json:"name"`
        Arguments string `json:"arguments"` // JSON string representation
    } `json:"function"`
}
```

### 1.2 The ReAct Loop (`RunAgentLoop`)
Add a new method `RunAgentLoop(ctx, messages, tools, executor)` that handles the multi-turn conversation automatically.

**Algorithm:**
1. Send `QwenRequest` with `Messages` and `Tools` to Qwen.
2. If the response `Choices[0].Message.ToolCalls` is empty, return the `Content` (The agent has finished reasoning).
3. If `ToolCalls` are present:
   - Append the Assistant's message (with `ToolCalls`) to the `messages` array.
   - For each tool call, execute the function via the `ToolExecutor`.
   - Create a new `QwenMessage` with `Role: "tool"`, `ToolCallID: id`, and `Content: <tool_result>`.
   - Append tool response messages to the array.
   - Loop back to step 1 (Limit loop to `max_depth = 5` to prevent infinite loops).

---

## 2. Tool Registry (`backend/internal/services/agent_tools.go`)

Create a new file to act as the bridge between the Agent and the Databases.

### 2.1 ToolExecutor Interface
```go
type ToolExecutor interface {
    Execute(ctx context.Context, name string, arguments string) (string, error)
}
```

### 2.2 QuillExecutor Implementation
Create `QuillExecutor` struct containing pointers to `VectorRepo`, `GraphRepo`, and `MemoryService`. 

**Implement `Execute()` to route to these specific tools:**

#### Tool: `search_vector_memory`
- **Description:** "Searches the vector database for past events or lore mentions similar to the query."
- **Parameters:** `query` (string)
- **Logic:** Calls `QwenService.GenerateEmbedding(query)`, passes the embedding to `VectorRepo.FindSimilarEntity` or a new `FindSimilarParagraph` method to return raw text context from past chapters.

#### Tool: `query_entity_graph`
- **Description:** "Retrieves the relationship graph and status for a specific entity."
- **Parameters:** `entity_name` (string)
- **Logic:** Translates entity name to ID, calls `GraphRepo.GetNeighbors`, and formats the result into a readable string (e.g., "John is ALLY_OF Mary. John status is DECEASED.").

---

## 3. Intelligent Contradictions (`backend/internal/services/contradiction_service.go`)

Replace the static 120-character check with the new Agent Loop.

### 3.1 Refactoring `CheckSemantic`
Instead of sending a single prompt with static DB descriptions, initialize the Agent Loop.

**System Prompt:**
> "You are a Narrative Continuity AI. Analyze the following paragraph for logical contradictions. You MUST use the `search_vector_memory` tool to cross-reference claims in the paragraph against past lore before making a decision. If a character is mentioned, use `query_entity_graph` to check their status. Return a JSON array of contradictions only after investigating."

**Execution:**
- The paragraph is passed as the User Message.
- The Agent will autonomously call `search_vector_memory("What happened to the amulet?")`.
- The Executor will return: *"In Chapter 3, the amulet was destroyed in the fire."*
- The Agent will reason: *"The new paragraph says the amulet is intact. This is a contradiction."* and output the final JSON.

---

## 4. Semantic Plot Holes (`backend/internal/services/plot_hole_service.go`)

Replace the simple integer subtraction (`gap >= 8`) with a semantic evaluation of stale nodes.

### 4.1 Hybrid Graph-Agent Workflow
1. **Query Graph (Fast Filter):** 
   Update the Cypher query to find all entities (nodes) where `last_mentioned_chapter` is older than `8` chapters AND `relevance_score > 0.5` (filtering out background characters).
2. **Agent Evaluation (Deep Check):**
   For each stale entity, send a prompt to the Agent:
   > "The entity '{entity.Name}' has not been mentioned recently. Use your tools to read the context of their last appearance. Is this a forgotten plot thread (Plot Hole) or was their arc naturally concluded?"
3. **Agent Action:**
   The agent uses `search_vector_memory` to read the scene where the entity was last seen. If they walked away into the sunset, it's not a plot hole. If they were trapped in a burning building and never mentioned again, the Agent flags it as a `Plot Hole`.

---

## 5. Intelligent Timeline Validation (`backend/internal/services/timeline_service.go`)

Replace the trivial mathematical check (`eventOrder > presentOrder`) with true LLM-based chronological reasoning.

### 5.1 Refactoring `ValidatePosition`
- **New Logic:** Instead of just checking index numbers, extract the stated timeframe from the text and compare it with the vector history using the Agent Loop.
- **Agent Prompt:**
  > "Evaluate the following timeline event for logical chronological inconsistencies based on distance, travel time, aging, or cause-and-effect. Use `search_vector_memory` to find established rules (e.g., 'travel time from X to Y')."
- **Expected Outcome:** The agent can now detect complex temporal contradictions (e.g., "The journey takes 30 days, but the character arrived in 2 days").

---

## 6. Document Ingestion Pipeline (`backend/internal/services/ingestion_service.go`)

Implement the completely missing `[REQ-F10]` feature to allow users to upload past books. This is critical for an impressive Hackathon Demo.

### 6.1 Creating the Ingestion Service
Create `ingestion_service.go` and `ingestion_handler.go`.

**The Pipeline (Async Goroutine):**
1. **Parse & Chunk:** Read the uploaded `.md` or `.txt` file and split it by Markdown Headers (e.g., `# Chapter 1`) to create `Chapter` records.
2. **Text Processing:** Break chapters into paragraphs.
3. **Entity Extraction:** Run Qwen-Turbo over the chunks to extract all characters, places, and rules.
4. **Vector Embeddings:** Generate embeddings for every paragraph and save them via `VectorRepo` to `pgvector`.
5. **Graph Construction:** Populate Apache AGE with the extracted relationships.
6. **WebSocket Updates:** Emit `ingestion_progress` events via WebSocket so the frontend displays a loading bar.

---

## 7. Security Patch: Cypher Injection (`backend/internal/repositories/graph_repo.go`)

**Vulnerability:** The current code uses `fmt.Sprintf` to concatenate entity names into Apache AGE Cypher queries. A name like `King's Landing` will break the SQL syntax and crash the pipeline.

### 7.1 Refactoring `CreateNode` and `CreateEdge`
Apache AGE does not support standard `pgx` `$1, $2` parameters inside the `$$ $$` Cypher block easily.
We must implement a strict sanitization function for all properties before injecting them into the Cypher query.

```go
func escapeCypherString(val string) string {
    // Escape single quotes and backslashes for AGE properties
    v := strings.ReplaceAll(val, `\`, `\\`)
    v = strings.ReplaceAll(v, `'`, `\'`)
    return v
}
```
Update all `fmt.Sprintf` calls in `graph_repo.go` to wrap string properties in `escapeCypherString()`.
