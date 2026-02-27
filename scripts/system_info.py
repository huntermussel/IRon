#!/usr/bin/env python3
import argparse
import os
import platform
import shutil
import sys
from datetime import datetime

# ToolSchema: {"description": "Get system performance metrics including disk, memory, OS info and load average.", "parameters": {"type": "object", "properties": {}}}


def get_stats():
    """
    Collects basic system health and information metrics using standard libraries.
    """
    try:
        # Disk Usage
        total, used, free = shutil.disk_usage("/")
        used_pct = (used / total) * 100

        # Memory (Linux/macOS specific fallback for basic stats if psutil is missing)
        # For simplicity and zero-dependency, we stick to what shutil and os provide

        stats = [
            "=== System Health Report ===",
            f"Timestamp: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}",
            f"OS: {platform.system()} {platform.release()}",
            f"Platform: {platform.platform()}",
            f"Architecture: {platform.machine()}",
            f"Processor: {platform.processor()}",
            "",
            "--- Storage (Root /) ---",
            f"Total: {total // (2**30)} GB",
            f"Used:  {used // (2**30)} GB ({used_pct:.1f}%)",
            f"Free:  {free // (2**30)} GB",
            "",
            "--- Performance ---",
        ]

        # Load Average (Not available on Windows)
        if hasattr(os, "getloadavg"):
            load1, load5, load15 = os.getloadavg()
            stats.append(
                f"Load Average (1m, 5m, 15m): {load1:.2f}, {load5:.2f}, {load15:.2f}"
            )
        else:
            stats.append("Load Average: Not available on this platform")

        # Basic CPU Count
        stats.append(f"CPU Cores: {os.cpu_count()}")

        return "\n".join(stats)
    except Exception as e:
        return f"Error collecting system information: {e}"


if __name__ == "__main__":
    # No arguments needed for this tool but using argparse for consistency with IRon's bridge
    parser = argparse.ArgumentParser(description="System Information Tool for IRon")
    args = parser.parse_args()

    print(get_stats())
