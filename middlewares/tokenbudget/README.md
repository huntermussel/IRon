## token_budget

- **What it does**: Caps `LLMParams.MaxTokens` to the provided budget to reduce cost.
- **When it runs**: `before_llm_request` (priority 90).
- **Controls**: `Event.Context["token_budget"]=int` (required to apply).
- **Toggle**: Always loaded; no effect if budget is absent.
