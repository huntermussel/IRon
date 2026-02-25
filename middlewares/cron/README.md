# Cron Middleware

This middleware handles recurring task scheduling using the shared NLU engine.

## Intents

- **set_cron**

## Utterances

- "remind me to {action} every {duration}"
- "schedule {action} every {duration}"
- "run {action} every {duration}"
- "execute {action} every {duration}"
- "do {action} every {duration}"

## Example

User: "remind me to take water every 2 hours"
Response: "Scheduled 'take water' every 2 hours (Cron: 0 */2 * * *)."
Instruction: To install, run: (crontab -l 2>/dev/null; echo "0 */2 * * * notify-send 'take water'") | crontab -
