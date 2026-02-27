#!/usr/bin/env python3
import argparse
import os
from datetime import datetime

from google.auth.transport.requests import Request
from google.oauth2.credentials import Credentials
from google_auth_oauthlib.flow import InstalledAppFlow
from googleapiclient.discovery import build

# ToolSchema: {"description": "List upcoming events from Google Calendar.", "parameters": {"type": "object", "properties": {"count": {"type": "integer", "description": "Number of events to fetch", "default": 10}}}}

# If modifying these scopes, delete the file token.json.
SCOPES = ["https://www.googleapis.com/auth/calendar.readonly"]


def get_creds():
    """
    Handles Google API authentication.
    Expects 'credentials.json' in the root directory.
    Saves/loads user tokens in 'token.json'.
    """
    creds = None
    if os.path.exists("token.json"):
        creds = Credentials.from_authorized_user_file("token.json", SCOPES)

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

        with open("token.json", "w") as token:
            token.write(creds.to_json())
    return creds


def list_events(count):
    """
    Lists the next 'count' events from the user's primary calendar.
    """
    try:
        creds = get_creds()
        service = build("calendar", "v3", credentials=creds)

        # 'Z' indicates UTC time
        now = datetime.utcnow().isoformat() + "Z"

        events_result = (
            service.events()
            .list(
                calendarId="primary",
                timeMin=now,
                maxResults=count,
                singleEvents=True,
                orderBy="startTime",
            )
            .execute()
        )
        events = events_result.get("items", [])

        if not events:
            return "No upcoming events found."

        output = [f"Upcoming {len(events)} events:", "=" * 40]
        for event in events:
            start = event["start"].get("dateTime", event["start"].get("date"))
            summary = event.get("summary", "(No Summary)")
            description = event.get("description", "")
            location = event.get("location", "")

            event_str = f"- {start}: {summary}"
            if location:
                event_str += f" @ {location}"
            output.append(event_str)
            if description:
                output.append(f"  Info: {description[:100]}...")

        return "\n".join(output)
    except Exception as e:
        return f"Error accessing Google Calendar: {str(e)}"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Google Calendar Tool for IRon")
    parser.add_argument(
        "--count", type=int, default=10, help="Number of events to fetch"
    )
    args = parser.parse_args()

    print(list_events(args.count))
