#!/usr/bin/env python3
import argparse
import base64
import os

from google.oauth2.credentials import Credentials
from googleapiclient.discovery import build

# ToolSchema: {"description": "Read the full content of a specific Gmail message by ID.", "parameters": {"type": "object", "properties": {"message_id": {"type": "string", "description": "The unique Gmail message ID"}}, "required": ["message_id"]}}


def read_email(msg_id):
    """
    Retrieves the full body of a specific email.
    Requires 'token.json' created by gmail_inbox.py.
    """
    if not os.path.exists("token.json"):
        return "Error: Authentication token not found. Please run a listing tool (like py_gmail_inbox) first to authenticate."

    try:
        creds = Credentials.from_authorized_user_file("token.json")
        service = build("gmail", "v1", credentials=creds)

        message = (
            service.users()
            .messages()
            .get(userId="me", id=msg_id, format="full")
            .execute()
        )

        payload = message.get("payload", {})
        headers = payload.get("headers", [])

        subject = next(
            (h["value"] for h in headers if h["name"] == "Subject"), "(No Subject)"
        )
        sender = next(
            (h["value"] for h in headers if h["name"] == "From"), "(Unknown Sender)"
        )

        # Extract body
        body = ""
        if "parts" in payload:
            for part in payload["parts"]:
                if part["mimeType"] == "text/plain":
                    data = part["body"].get("data")
                    if data:
                        body = base64.urlsafe_b64decode(data).decode("utf-8")
                        break
        else:
            data = payload.get("body", {}).get("data")
            if data:
                body = base64.urlsafe_b64decode(data).decode("utf-8")

        if not body:
            body = message.get("snippet", "No plain text body found.")

        output = [
            f"=== Email Content (ID: {msg_id}) ===",
            f"From: {sender}",
            f"Subject: {subject}",
            "=" * 40,
            body,
            "=" * 40,
        ]

        return "\n".join(output)
    except Exception as e:
        return f"Error reading email {msg_id}: {str(e)}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Gmail Message Reader Tool for IRon")
    parser.add_argument(
        "--message_id", required=True, help="The unique Gmail message ID"
    )
    args = parser.parse_args()

    print(read_email(args.message_id))
