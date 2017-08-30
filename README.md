# Bot to manage and test GitHub pull requests

The PR bot processes GitHub webhook events and can be built with `go get github.com/macports/mpbot-github/pr/prbot`.

The CI bot tests PRs on Travis CI and can be built with `go get github.com/macports/mpbot-github/ci/runner`.

## PR bot

To run the PR bot, you need to add a webhook to your `macports-ports` repository. The webhook must have a secret and receive at least `issue_comment` (Issue comment), `pull_request` (Pull request), `pull_request_review` (Pull request review) events.

You also need a GitHub OAuth2 access token (e.g. a [personal access tokens](https://github.com/settings/tokens)) as `HUB_BOT_SECRET` below.

You also need to set the following environment variables:

In `pr/db/dbutil.go`, [connection strings](https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters) for the PostgreSQL databases:

- `TRAC_DB`: connection string for the Trac DB
- `WWW_DB`: connection string for the PortIndex DB (https://github.com/macports/macports-infrastructure/tree/master/jobs)
- `PR_DB`: connection string for the bot's own DB

In `pr/prbot/main.go`, secrets and production flag:

- `HUB_WEBHOOK_SECRET`: used to verify webhook events
- `HUB_BOT_SECRET`: used to comment and modify labels in PRs
- `BOT_ENV`: set to `production` to actually mention maintainers (e.g. @l2dy instead of @_l2dy)

You also need a database with port maintainers and Trac account emails. We have a [script](https://github.com/macports/macports-infrastructure/blob/master/jobs/portindex2postgres.tcl) that generates PostgreSQL dump from all ports in your local MacPorts installation for use in [www.macports.org](https://www.macports.org/ports.php) and the PR bot uses the `maintainers` table generated. The schema of Trac account emails is shown below:

```
--
-- Name: session_attribute; Type: TABLE; Schema: trac_macports; Owner: trac; Tablespace:
--

CREATE TABLE session_attribute (
    sid text NOT NULL,
    authenticated integer NOT NULL,
    name text NOT NULL,
    value text
);

--
-- Name: session_attribute_pk; Type: CONSTRAINT; Schema: trac_macports; Owner: trac; Tablespace:
--

ALTER TABLE ONLY session_attribute
    ADD CONSTRAINT session_attribute_pk PRIMARY KEY (sid, authenticated, name);
```

And some test data:

```
l2dy	1	email	l2dy@macports.org
l2dy	1	name	Zero King
l2dy	1	tz	UTC
```

You can use `-l addr:port` to set the listen address for GitHub webhook. It defaults to `:8081`, which means any address and port 8081.

## CI bot

To run the CI bot, you need to have the `.travis.yml` and `_ci/*` files in your `macports-ports` repository and enable Travis CI for that repository [here](https://travis-ci.org/profile).

The CI bot is executed as root under Travis CI environment bootstrapped by [this script](https://github.com/macports/macports-ports/blob/master/_ci/bootstrap.sh).

It assumes that MacPorts is already installed and no cleanup is needed.

It will try to upload logs to https://paste.macports.org/, hardcoded in [remoteLog.go](https://github.com/macports-staging/mpbot-github/blob/doc/ci/logger/remoteLog.go).

For local debugging, you have to use your own [Mojopaste](https://metacpan.org/pod/App::mojopaste) instance and run the bot in a clone of `macports-ports`. The bot will try to get the list of changed ports from `macports/master...HEAD` so you need to fetch the master branch of the `macports` remote and commit your changes on the current branch.
