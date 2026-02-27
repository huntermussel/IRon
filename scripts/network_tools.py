#!/usr/bin/env python3
import argparse
import socket
import subprocess
import urllib.error
import urllib.request

# ToolSchema: {"description": "Network utilities: ping a host, check port status, or perform a simple HTTP GET request.", "parameters": {"type": "object", "properties": {"action": {"type": "string", "enum": ["ping", "port_check", "http_get"], "description": "Network action to perform"}, "target": {"type": "string", "description": "Hostname, IP, or URL"}, "port": {"type": "integer", "description": "Port number (for port_check)"}}, "required": ["action", "target"]}}


def ping_host(target):
    try:
        # Use -c 4 for Linux/macOS and -n 4 for Windows
        cmd = ["ping", "-c", "4", target]
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
        if result.returncode == 0:
            return result.stdout.strip()
        else:
            return f"Ping failed:\n{result.stderr.strip() or result.stdout.strip()}"
    except subprocess.TimeoutExpired:
        return f"Ping to {target} timed out after 10 seconds."
    except FileNotFoundError:
        # Fallback if ping command is not found
        return "Error: 'ping' command not found on the host system."
    except Exception as e:
        return f"Unexpected error during ping: {e}"


def check_port(target, port):
    if not port:
        return "Error: Port number is required for port_check."

    try:
        # Create a socket and attempt to connect
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.settimeout(3.0)  # 3 second timeout
            result = s.connect_ex((target, int(port)))
            if result == 0:
                return f"Port {port} on {target} is OPEN."
            else:
                return f"Port {port} on {target} is CLOSED or FILTERED (Error code: {result})."
    except socket.gaierror as e:
        return f"Hostname resolution failed for {target}: {e}"
    except Exception as e:
        return f"Error checking port {port} on {target}: {e}"


def http_get(url):
    if not url.startswith(("http://", "https://")):
        url = "http://" + url

    try:
        req = urllib.request.Request(
            url, headers={"User-Agent": "Mozilla/5.0 (compatible; IRonNetworkTool/1.0)"}
        )
        with urllib.request.urlopen(req, timeout=10) as response:
            status = response.getcode()
            content_type = response.headers.get("Content-Type", "unknown")
            body = response.read()

            # Try to decode text content
            text_content = ""
            if "text" in content_type or "json" in content_type:
                try:
                    text_content = body.decode("utf-8")
                    # Truncate to avoid flooding context window
                    if len(text_content) > 2000:
                        text_content = (
                            text_content[:2000] + "\n\n... [Content Truncated]"
                        )
                except UnicodeDecodeError:
                    text_content = "[Binary or un-decodable data]"
            else:
                text_content = f"[{len(body)} bytes of {content_type} data]"

            return (
                f"Status: {status} OK\nContent-Type: {content_type}\n\n{text_content}"
            )

    except urllib.error.HTTPError as e:
        return f"HTTP Error: {e.code} {e.reason}"
    except urllib.error.URLError as e:
        return f"URL Error: Failed to reach {url}. Reason: {e.reason}"
    except Exception as e:
        return f"Unexpected error fetching {url}: {e}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Network Tools for IRon")
    parser.add_argument(
        "--action", choices=["ping", "port_check", "http_get"], required=True
    )
    parser.add_argument("--target", required=True, help="Hostname, IP, or URL")
    parser.add_argument("--port", type=int, help="Port number for port_check")

    args = parser.parse_args()

    if args.action == "ping":
        print(ping_host(args.target))
    elif args.action == "port_check":
        print(check_port(args.target, args.port))
    elif args.action == "http_get":
        print(http_get(args.target))
