# cli

Common Fate CLI package.

## Getting started

Log in:

```
go run cmd/main.go login
```

List access rules:

```
go run cmd/main.go rules list
```

## API context

When logging in, the CLI package creates a file at `~/.commonfate/config` which contains the following:

```
current_context = "default"

[context]
  [context.default]
    dashboard_url = "https://commonfate.example.com"
```

Where the `dashboard_url` field is the frontend dashboard URL for a Common Fate deployment.

To determine the backend API URL, the CLI queries for the `aws-exports.json` file in the dashboard - e.g. https://commonfate.example.com/aws-exports.json. This JSON file contains the API URL, and is the same file that is used by our frontend web app to determine the API URL.

You can find this logic in `pkg/config` in this repository.
