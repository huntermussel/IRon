#!/usr/bin/env python3
import argparse
import os
from pathlib import Path

# ToolSchema: {"description": "Generate a compact tree-view map of the repository structure with file sizes to help the AI understand the project layout.", "parameters": {"type": "object", "properties": {"root": {"type": "string", "description": "Directory to map (defaults to current directory)"}, "depth": {"type": "integer", "description": "Max depth to recurse", "default": 3}}, "required": ["root"]}}


def generate_map(root_path, max_depth=3, current_depth=0):
    if current_depth > max_depth:
        return ""

    output = []
    try:
        p = Path(root_path)
        if not p.exists():
            return f"Error: Path {root_path} does not exist."

        # Sort items: directories first, then files
        items = sorted(p.iterdir(), key=lambda x: (not x.is_dir(), x.name.lower()))

        for item in items:
            # Skip common noise directories and hidden files
            if item.name.startswith(".") or item.name in {
                "node_modules",
                "vendor",
                "bin",
                "dist",
                "__pycache__",
                "venv",
            }:
                continue

            indent = "  " * current_depth
            if item.is_dir():
                output.append(f"{indent}📁 {item.name}/")
                subdir_map = generate_map(item, max_depth, current_depth + 1)
                if subdir_map:
                    output.append(subdir_map)
            else:
                size_kb = item.stat().st_size / 1024
                output.append(f"{indent}📄 {item.name} ({size_kb:.1f} KB)")

    except Exception as e:
        return f"Error accessing {root_path}: {e}"

    return "\n".join(filter(None, output))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Repository Mapper Tool for IRon")
    parser.add_argument("--root", default=".", help="Root directory to map")
    parser.add_argument("--depth", type=int, default=3, help="Maximum recursion depth")
    args = parser.parse_args()

    # If the LLM passes "." or empty, use current working directory
    root_dir = args.root if args.root and args.root != "" else "."

    print(f"Project Map for: {os.path.abspath(root_dir)}\n" + "=" * 40)
    result = generate_map(root_dir, args.depth)
    print(result if result else "Directory is empty or inaccessible.")
