## greeting

- **What it does**: Detects simple salutations and replies with “Hi, how can I assist you today?” without calling the LLM (saves tokens).
- **When it runs**: `before_llm_request` (priority 110).
- **Toggle**: `Event.Context["greeting"]=false` to disable; otherwise enabled.
