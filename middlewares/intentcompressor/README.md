## intent_compressor

- **What it does**: Abbreviates verbose requests into short intent strings (e.g., `react landing: institutional`) and keeps constraint flag `cstr` when negations appear.
- **When it runs**: `before_llm_request` (priority 90).
- **Controls**: `Event.Context["intent_compressor_mode"]="safe|aggr"` (default aggr); `Event.Context["intent_compressor_min_score"]=int`.
- **Toggle**: `Event.Context["intent_compressor"]=false` to disable; default enabled.
