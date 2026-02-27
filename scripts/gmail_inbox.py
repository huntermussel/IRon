#!/usr/bin/env python3
import argparse
import os

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow
from googleapiclient.discovery import build

# ToolSchema: {"description": "List latest emails from Gmail inbox.", "parameters": {"type": "object", "properties": {"count": {"type": "integer", "description": "Number of emails to fetch", "default": 5}, "label": {"type": "string", "description": "Label to filter (INBOX, UNREAD, SENT)", "default": "INBOX"}}}}

# If modifying these scopes, delete the file token.json.
SCOPES = ["https://www.googleapis.com/auth/gmail.readonly"]


def get_creds():
    """
    Handles Google API authentication.
    Expects 'credentials.json' in the root directory.
    Saves/loads user tokens in 'token.json'.
    """
    creds = None
    # The file token.json stores the user's access and refresh tokens.
    if os.path.exists("token.json"):
        creds = Credentials.from_authorized_user_file("token.json", SCOPES)

    # If there are no (valid) credentials available, let the user log in.
    if not creds or not creds.valid:
        if creds and creds.expired and creds.refresh_token:
            try:
                creds.refresh(Request())
            except Exception:
                os.remove("token.json")
                return get_creds()
        else:
            if not os.path.exists("credentials.json"):
                raise FileNotFoundError(
                    "Missing 'credentials.json'. Please provide it in the root directory."
                )

            flow = InstalledAppFlow.from_client_secrets_file("credentials.json", SCOPES)
            creds = flow.run_local_server(port=0)

        # Save the credentials for the next run
        with open("token.json", "w") as token:
            token.write(creds.to_json())
    return creds


def list_emails(count, label):
    """
    Fetches the list of recent emails.
    """
    try:
        creds = get_creds()
        service = build("gmail", "v1", credentials=creds)

        results = (
            service.users()
            .messages()
            .list(userId="me", maxResults=count, labelIds=[label])
            .execute()
        )

        messages = results.get("messages", [])

        if not messages:
            return f"No messages found with label '{label}'."

        output = [f"Found {len(messages)} recent emails in {label}:", "=" * 40]

        for msg in messages:
            # Get basic header info for the list
            m = (
                service.users()
                .messages()
                .get(
                    userId="me",
                    id=msg["id"],
                    format="metadata",
                    metadataHeaders=["Subject", "From", "Date"],
                )
                .execute()
            )

            headers = m.get("payload", {}).get("headers", [])
            subject = next(
                (h["value"] for h in headers if h["name"] == "Subject"), "(No Subject)"
            )
            sender = next(
                (h["value"] for h in headers if h["name"] == "From"), "(Unknown Sender)"
            )
            date = next(
                (h["value"] for h in headers if h["name"] == "Date"), "(No Date)"
            )

            output.append(f"ID: {msg['id']}")
            output.append(f"From: {sender}")
            output.append(f"Date: {date}")
            output.append(f"Subject: {subject}")
            output.append(f"Snippet: {m.get('snippet', '')}")
            output.append("-" * 20)

        return "\n".join(output)
    except Exception as e:
        return f"Error accessing Gmail: {str(e)}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Gmail Inbox Tool for IRon")
    parser.add_argument(
        "--count", type=int, default=5, help="Number of emails to fetch"
    )
    parser.add_argument(
        "--label", default="INBOX", help="Gmail label (e.g., INBOX, UNREAD, SENT)"
    )
    args = parser.parse_args()

    print(list_emails(args.count, args.label))
