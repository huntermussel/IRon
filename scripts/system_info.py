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

        # Memory stats (Linux fallback)
        mem_info = "Memory Info: Only detailed on Linux"
        if platform.system() == "Linux":
            try:
                with open("/proc/meminfo", "r") as f:
                    lines = f.readlines()
                mem = {}
                for line in lines:
                    parts = line.split(":")
                    if len(parts) == 2:
                        mem[parts[0].strip()] = int(parts[1].split()[0].strip())

                total_mem = mem.get("MemTotal", 0) / 1024  # MB
                avail_mem = mem.get("MemAvailable", mem.get("MemFree", 0)) / 1024  # MB
                used_mem = total_mem - avail_mem
                mem_info = f"{used_mem / 1024:.1f} GB used / {total_mem / 1024:.1f} GB total ({avail_mem / 1024:.1f} GB available)"
            except Exception:
                pass

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
            "--- Memory ---",
            mem_info,
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
