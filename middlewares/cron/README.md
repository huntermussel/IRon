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
- "{action} every {duration}" (e.g. "search every hour")
- "{action} {count} times per {unit}" (e.g. "search 1 time per week")
- "on {duration} {action}" (e.g. "on mondays backup database")

## Example

User: "remind me to take water every 2 hours"
Response: "Scheduled 'take water' every 2 hours (Cron: 0 */2 * * *)."
Instruction: To install, run: (crontab -l 2>/dev/null; echo "0 */2 * * * notify-send 'take water'") | crontab -

User: "search 1 time per week"
Response: "Scheduled 'search' every 1 times per week (Cron: 0 0 * * 0)."
Instruction: To install, run: (crontab -l 2>/dev/null; echo "0 0 * * 0 notify-send 'search'") | crontab -

User: "on mondays backup database"
Response: "Scheduled 'backup database' every mondays (Cron: 0 0 * * 1)."
Instruction: To install, run: (crontab -l 2>/dev/null; echo "0 0 * * 1 notify-send 'backup database'") | crontab -
