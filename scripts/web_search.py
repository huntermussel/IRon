#!/usr/bin/env python3
import argparse
import json
import urllib.error
import urllib.parse
import urllib.request

# ToolSchema: {"description": "Search the web using DuckDuckGo to find latest information.", "parameters": {"type": "object", "properties": {"query": {"type": "string", "description": "The search query"}}, "required": ["query"]}}


def search(query):
    # Using DuckDuckGo Lite to avoid strict bot detection and BeautifulSoup parsing
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
    }

    # We use a public alternative endpoint or basic duckduckgo lite parsing using regex/string ops since BS4 is removed
    # Actually, DuckDuckGo Lite is hard to parse reliably without BS4.
    # Let's use a simpler approach: querying a public DuckDuckGo-like API or doing rudimentary text extraction

    url = "https://html.duckduckgo.com/html/?q=" + urllib.parse.quote(query)
    req = urllib.request.Request(url, headers=headers)

    try:
        with urllib.request.urlopen(req, timeout=10) as response:
            html = response.read().decode("utf-8", errors="ignore")

            # Very rudimentary parsing without BS4
            results = []
            parts = html.split('class="result__snippet')

            for part in parts[
                1:6
            ]:  # Skip first split (before any results), get up to 5
                # Extract snippet
                snippet_end = part.find("</a>")
                if snippet_end == -1:
                    continue
                snippet = (
                    part[part.find(">") + 1 : snippet_end]
                    .replace("<b>", "")
                    .replace("</b>", "")
                    .strip()
                )

                # Look backwards for the title and link (which precede the snippet)
                results.append(f"Result Snippet: {snippet}\n---")

            if not results:
                return "No results found or rate limited by search provider."

            return "\n".join(results)

    except urllib.error.HTTPError as e:
        return f"Search HTTP Error: {e.code} - {e.reason}"
    except urllib.error.URLError as e:
        return f"Search Connection Error: {e.reason}"
    except Exception as e:
        return f"Error performing web search: {e}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="DuckDuckGo Search Tool for IRon")
    parser.add_argument("--query", required=True, help="Search query")
    args = parser.parse_args()

    print(search(args.query))
