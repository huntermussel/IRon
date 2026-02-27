#!/usr/bin/env python3
import argparse
import os
import subprocess
import sys

# ToolSchema: {"description": "Manage software packages using pip (Python) or npm (Node.js).", "parameters": {"type": "object", "properties": {"manager": {"type": "string", "enum": ["pip", "npm"], "description": "Package manager to use"}, "action": {"type": "string", "enum": ["install", "uninstall", "list"], "description": "Action to perform"}, "package": {"type": "string", "description": "Name of the package (required for install/uninstall)"}}, "required": ["manager", "action"]}}


def run_command(cmd_list):
    try:
        # Run command and capture output to prevent polluting the terminal/context massively
        result = subprocess.run(cmd_list, capture_output=True, text=True, check=True)
        out = result.stdout.strip()

        # Truncate extremely long outputs (like pip list or npm install logs)
        if len(out) > 2000:
            out = out[:2000] + "\n... [Output truncated for brevity]"

        return f"Success:\n{out}" if out else "Success (No output)"
    except subprocess.CalledProcessError as e:
        err_msg = e.stderr.strip() if e.stderr else e.stdout.strip()
        return f"Error (Exit code {e.returncode}):\n{err_msg}"
    except FileNotFoundError:
        return f"Error: The package manager '{cmd_list[0]}' is not installed or not in PATH."
    except Exception as e:
        return f"Unexpected error: {e}"


def handle_pip(action, package):
    # Determine the correct pip executable
    pip_cmd = "pip"
    if sys.platform != "win32" and os.system("command -v pip3 >/dev/null 2>&1") == 0:
        pip_cmd = "pip3"

    if action == "list":
        return run_command([pip_cmd, "list"])

    if not package:
        return "Error: 'package' parameter is required for install/uninstall."

    if action == "install":
        return run_command([pip_cmd, "install", package])
    elif action == "uninstall":
        return run_command([pip_cmd, "uninstall", "-y", package])


def handle_npm(action, package):
    if action == "list":
        return run_command(["npm", "list", "--depth=0"])

    if not package:
        return "Error: 'package' parameter is required for install/uninstall."

    if action == "install":
        return run_command(["npm", "install", package])
    elif action == "uninstall":
        return run_command(["npm", "uninstall", package])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Package Manager Tool for IRon")
    parser.add_argument("--manager", choices=["pip", "npm"], required=True)
    parser.add_argument(
        "--action", choices=["install", "uninstall", "list"], required=True
    )
    parser.add_argument("--package", default="", help="Name of the package")

    args = parser.parse_args()

    if args.manager == "pip":
        print(handle_pip(args.action, args.package))
    elif args.manager == "npm":
        print(handle_npm(args.action, args.package))
