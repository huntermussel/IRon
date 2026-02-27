#!/usr/bin/env python3
import argparse
import os
import signal
import subprocess
import sys

# ToolSchema: {"description": "Manage system processes: list running processes or kill them by PID.", "parameters": {"type": "object", "properties": {"action": {"type": "string", "enum": ["list", "kill"], "description": "Action to perform"}, "filter": {"type": "string", "description": "Filter process list by name or keyword (for 'list' action)"}, "pid": {"type": "integer", "description": "Process ID to kill (for 'kill' action)"}}, "required": ["action"]}}


def list_processes(keyword=""):
    try:
        # Use standard ps command for cross-platform compatibility (Linux/macOS)
        result = subprocess.run(
            ["ps", "aux"], capture_output=True, text=True, check=True
        )
        lines = result.stdout.strip().split("\n")

        if not lines:
            return "No processes found."

        header = lines[0]
        output = [header]

        for line in lines[1:]:
            if keyword.lower() in line.lower():
                output.append(line)

        # Limit output to avoid token overflow
        if len(output) > 25:
            return (
                "\n".join(output[:25])
                + f"\n... (and {len(output) - 25} more, refine filter to see them)"
            )

        return "\n".join(output)
    except Exception as e:
        return f"Error listing processes: {e}"


def kill_process(pid):
    if not pid:
        return "Error: PID is required for kill action."

    try:
        os.kill(pid, signal.SIGTERM)
        return f"Successfully sent SIGTERM to PID {pid}."
    except ProcessLookupError:
        return f"Error: No process found with PID {pid}."
    except PermissionError:
        return f"Error: Permission denied to kill PID {pid}."
    except Exception as e:
        return f"Error killing process {pid}: {e}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Process Manager Tool for IRon")
    parser.add_argument("--action", choices=["list", "kill"], required=True)
    parser.add_argument("--filter", default="", help="Filter string for list action")
    parser.add_argument("--pid", type=int, help="Process ID to kill")

    args = parser.parse_args()

    if args.action == "list":
        print(list_processes(args.filter))
    elif args.action == "kill":
        print(kill_process(args.pid))
