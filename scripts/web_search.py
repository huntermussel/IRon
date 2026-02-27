#!/usr/bin/env python3
import argparse
import sys

import requests
from bs4 import BeautifulSoup

# ToolSchema: {"description": "Search the web using DuckDuckGo to find latest information.", "parameters": {"type": "object", "properties": {"query": {"type": "string", "description": "The search query"}}, "required": ["query"]}}


def search(query):
    headers = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36"
    }
    url = f"https://html.duckduckgo.com/html/?q={query}"
    try:
        res = requests.get(url, headers=headers, timeout=10)
        res.raise_for_status()
        soup = BeautifulSoup(res.text, "html.parser")
        results = []

        # DuckDuckGo HTML version uses result__a and result__snippet classes
        for result in soup.find_all("div", class_="result")[:5]:
            title_tag = result.find("a", class_="result__a")
            snippet_tag = result.find("a", class_="result__snippet")

            if title_tag:
                title = title_tag.get_text()
                link = title_tag["href"]
                snippet = (
                    snippet_tag.get_text() if snippet_tag else "No snippet available."
                )
                results.append(f"Title: {title}\nLink: {link}\nSnippet: {snippet}\n")

        if not results:
            return "No results found. DuckDuckGo might be rate-limiting or the query returned no data."

        return "\n".join(results)
    except Exception as e:
        return f"Error performing web search: {e}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="DuckDuckGo Search Tool for IRon")
    parser.add_argument("--query", required=True, help="Search query")
    args = parser.parse_args()

    print(search(args.query))
