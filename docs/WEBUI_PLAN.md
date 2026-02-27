# IRon Web UI Architecture & Feature Plan

This document outlines the roadmap for migrating the current monolithic `index.html` Web UI to a modern, componentized front-end architecture using Vite + React (or Vue/Svelte), and implementing advanced AI features inspired by AutoGPT, OpenDevin, OpenWebUI, and others.

---

## Phase 1: Foundation (Vite + Componentization)

The goal of this phase is to establish a robust frontend development environment and componentize the existing UI to make it maintainable and extensible.

### 1. Project Setup
- [ ] Initialize a Vite project (e.g., inside `internal/webui/frontend` or `ui/`).
- [ ] Set up TailwindCSS for styling and Lucide/FontAwesome for icons.
- [ ] Configure the build process to output to `internal/webui/static` so `go:embed` still works seamlessly.
- [ ] Set up a development proxy in Vite to route API calls (`/api/*`) to the Go backend (`localhost:8080`) during development.

### 2. Component Architecture
Break down the monolithic HTML into reusable components:
- [ ] `Sidebar`: Navigation links and connection status.
- [ ] `TopBar`: Mobile menu toggle and current agent status.
- [ ] `DashboardView`: Stats, usage metrics, and feature toggles.
- [ ] `ChatView`: The main conversation interface.
    - [ ] `MessageList`: Renders user and AI messages.
    - [ ] `MessageBubble`: Individual message renderer with Markdown/Code highlighting support (e.g., using `react-markdown` and `prismjs`/`highlight.js`).
    - [ ] `ChatInput`: Text area, send button, and attachment/tool trigger buttons.
- [ ] `SettingsView`: Forms for LLM configuration, API keys, and middleware settings.

### 3. State Management
- [ ] Implement a global state store (e.g., Zustand, Redux, or Context API) to manage:
    - Current configuration (provider, model, keys).
    - Chat history and active session ID.
    - UI state (active tab, sidebar visibility).
    - Server connection status.

---

## Phase 2: Multi-Agent & Workspace Features

This phase introduces advanced session management and workspace capabilities, drawing inspiration from OpenWebUI and AutoGPT.

### 1. Multi-Agent / Multi-Session Support
- [ ] **Backend:** Modify `Gateway.InitService` and `Server` to support multiple `chat.Service` instances mapped to session IDs.
- [ ] **Frontend:** Add a "New Chat" button and a list of recent chats in the Sidebar.
- [ ] **Frontend:** Implement persona/agent selection per chat (e.g., "Coding Assistant", "General Chat", "Data Analyst").

### 2. Workspace & File Management
- [ ] **Backend:** Create APIs for listing, reading, and writing files in a dedicated workspace directory.
- [ ] **Frontend:** Add a "Workspace" or "Files" view alongside Chat.
- [ ] **Frontend:** Implement a split-pane view (Chat on left, Code/File Editor on right) similar to OpenDevin/Cursor.

### 3. Tool & Skill Management (Plugin UI)
- [ ] **Backend:** Expose an API endpoint listing available tools/skills and their dynamic configuration options.
- [ ] **Frontend:** Create a "Tools" or "Plugins" store page where users can enable/disable specific middlewares (e.g., Python Executor, GitHub integration, Web Search).
- [ ] **Frontend:** Allow configuring tool-specific settings (e.g., setting the notification method for the Timer tool: Desktop Notification, Telegram, or Web Audio alert).

---

## Phase 3: Advanced Autonomous Features (AutoGPT / OpenDevin inspired)

This phase focuses on observability, long-running tasks, and autonomous agent capabilities.

### 1. Execution Tracing & Transparency
- [ ] **Backend:** Stream intermediate "Thought" and "Action" events via Server-Sent Events (SSE) or WebSockets.
- [ ] **Frontend:** Enhance the `MessageBubble` to display a collapsible "Agent Thoughts" or "Tool Calls" section (e.g., *[Agent used 'shell' tool -> output: 'ls -la']*).
- [ ] **Frontend:** Add a "Stop/Cancel" button to interrupt long-running loops.

### 2. Agent "Memory" Management
- [ ] **Backend:** Create APIs to query, add, edit, and delete items from the Vector Memory Store.
- [ ] **Frontend:** Create a "Memory" settings tab where users can review what the AI "knows" about them and manually curate the context.

### 3. Task Planning & Sub-agents
- [ ] **Backend:** Implement a planner middleware that breaks down complex user prompts into sub-tasks.
- [ ] **Frontend:** Display a real-time progress tree/checklist of tasks the agent is currently working on.

### 4. Human-in-the-Loop (Approval workflow)
- [ ] **Backend:** Add a feature to pause execution and request user permission before running dangerous commands (e.g., `rm -rf`, modifying system configs).
- [ ] **Frontend:** Display interactive prompt dialogs inside the chat for "Approve" or "Deny" actions.

---

## Implementation Steps (Next Actions)

1. Create the `ui/` directory at the project root.
2. Run `npm create vite@latest . -- --template react-ts` (or your preferred framework).
3. Move the existing HTML/JS logic into the new component structure.
4. Update the Go `Makefile` to include a build step: `cd ui && npm run build && cp -r dist/* ../internal/webui/static/`.
5. Begin implementing Phase 2 backend APIs to support the richer frontend.