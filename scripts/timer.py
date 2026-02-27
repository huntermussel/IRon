#!/usr/bin/env python3
import argparse
import os
import subprocess
import sys
import time

# ToolSchema: {"description": "Set a timer or reminder that will notify you after a specified duration.", "parameters": {"type": "object", "properties": {"minutes": {"type": "number", "description": "Duration in minutes until the timer goes off"}, "message": {"type": "string", "description": "The reminder message to display"}}, "required": ["minutes", "message"]}}


def set_timer(minutes, message):
    if minutes <= 0:
        return "Error: Timer duration must be greater than 0."

    seconds = int(minutes * 60)

    # We want to spawn a background process that sleeps and then notifies.
    # This allows the IRon process to exit while the timer runs in the background.

    # Try to find a notification command
    notifier = None
    if sys.platform == "linux" or sys.platform == "linux2":
        notifier = "notify-send"
    elif sys.platform == "darwin":
        notifier = "osascript"
    elif sys.platform == "win32":
        # Basic fallback for Windows, though requires third-party or powershell
        pass

    if not notifier and sys.platform != "win32":
        return "Error: Could not determine a notification mechanism for this OS."

    try:
        if sys.platform == "linux" or sys.platform == "linux2":
            # Command: sleep X && notify-send "IRon Reminder" "Message"
            cmd = f'sleep {seconds} && notify-send "IRon Reminder" "{message}"'
            subprocess.Popen(
                cmd,
                shell=True,
                start_new_session=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        elif sys.platform == "darwin":
            # Command: sleep X && osascript -e 'display notification "Message" with title "IRon Reminder"'
            cmd = f'sleep {seconds} && osascript -e \'display notification "{message}" with title "IRon Reminder"\''
            subprocess.Popen(
                cmd,
                shell=True,
                start_new_session=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        else:
            return "Timers are currently only supported on Linux and macOS."

        return f"Timer set successfully. I will remind you '{message}' in {minutes} minutes."
    except Exception as e:
        return f"Failed to set timer: {e}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Timer / Reminder Tool for IRon")
    parser.add_argument("--minutes", type=float, required=True, help="Minutes to wait")
    parser.add_argument("--message", required=True, help="Message to display")

    args = parser.parse_args()
    print(set_timer(args.minutes, args.message))
