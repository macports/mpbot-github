# Bot to manage and test GitHub pull requests

The PR bot processes GitHub webhook events and can be built with `go get github.com/macports/mpbot-github/pr/prbot`.

The CI bot tests PRs on Travis CI and can be built with `go get github.com/macports/mpbot-github/ci/runner`.

## PR bot

To run the PR bot, you need to set the following environment variables.

In `pr/db/dbutil.go`, [connection strings](https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters) for the PostgreSQL databases:

- TRAC_DB: connection string for the Trac DB
- WWW_DB: connection string for the PortIndex DB (https://github.com/macports/macports-infrastructure/tree/master/jobs)
- PR_DB: connection string for the bot's own DB

In `pr/prbot/main.go`, secrets and production flag:

- HUB_WEBHOOK_SECRET: used to verify webhook events
- HUB_BOT_SECRET: used to comment and modify labels in PRs
- BOT_ENV: set to `production` to actually mention maintainers

You can use `-l addr:port` to set the listen address for GitHub webhook.

## CI bot

The CI bot is executed as root under Travis CI environment bootstrapped by [this script](https://github.com/macports/macports-ports/blob/master/_ci/bootstrap.sh).
It assumes that MacPorts is already installed and no cleanup is needed.
