## coding_tools_mode & coding_tools_exec

- **coding_tools_mode**: Injects minimal tool schemas (`ls`, `mkdir`, `find`, `diff`, `pwd`, `read_file`, `write_file`) and sets `ToolChoice=auto` for tool-calling models.
- **coding_tools_exec**: Executes the supported tools locally when tool calls are provided via `Event.Context["tool_calls"]`; returns outputs directly and cancels further processing.
- **When they run**: `coding_tools_mode` on `before_llm_request` (priority 85); `coding_tools_exec` on `after_llm_response` (priority 80).
- **Toggle**: `Event.Context["coding_tools_mode"]=false` to skip schema injection; `coding_tools_exec` activates only if `tool_calls` are present.
