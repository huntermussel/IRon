## trash_cleaner

- **What it does**: Removes low-signal words while preserving negations and technical tokens; replaces the user text with a compressed form.
- **When it runs**: `before_llm_request` (priority 100).
- **Toggle**: `Event.Context["trash_cleaner"]=false` to disable; default enabled.
