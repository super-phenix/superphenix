# Deployment

This directory contains a base Docker Compose setup for running the full console stack locally — useful for local development and testing without relying on a Kubernetes cluster.

It is **not** a production deployment. It bundles Kratos, Postgres, Permify, the Superphenix API, the auth UI, and the SPX panel into a single `docker compose up` workflow.

## What's here

- `docker-compose.yml` — the stack definition
- `.env.example` — user-specific paths you must set (frontend repo checkouts)
- `local/` - user-specifics configurations (the whole folder is gitignored)
- `kratos/` — Kratos config example and identity schema

## First-time setup

The stack expects local checkouts of the two frontend repos (`auth-ui`, `spx-panel`) and a local Kratos config.

```sh
# 1. Point the stack at your local frontend checkouts
cp .env.example .env
$EDITOR .env   # set AUTH_UI_PATH and SPX_PANEL_PATH to absolute paths

# 2. Create a local Kratos config (the example is committed, the real file is gitignored)
cp kratos/kratos.example.yml local/kratos/kratos.yml
$EDITOR local/kratos/kratos.yml   # set secrets.default (e.g. `openssl rand -base64 32`) and SMTP creds

# 3. Bring the stack up (from this directory)
docker compose up -d

# Or from root directory (and build option)
docker compose -f deployment/docker-compose.yml up -d --build
```

## Docker Compose Profiles

Every service in the stack is assigned to a **profile**. Only the profiles you activate will be started.

| Profile     | Services                                                                                    |
|-------------|---------------------------------------------------------------------------------------------|
| `backend`   | `kratos-migrate`, `kratos`, `postgresd`, `superphenix-api-db`, `permify`, `superphenix-api` |
| `auth-ui`   | `auth-ui`                                                                                   |
| `spx-panel` | `spx-panel`                                                                                 |

### Activating profiles via `COMPOSE_PROFILES`

The recommended way is to set the `COMPOSE_PROFILES` variable in your `.env` file (comma-separated list):

```dotenv
# .env — start everything
COMPOSE_PROFILES=backend,auth-ui,spx-panel
```

Then run the usual command without any `--profile` flags:

```sh
# From this directory
docker compose up -d

# From root directory
docker compose -f hacks/compose/docker-compose.yml up -d --build
```

To exclude a service, simply remove its profile from the list. For example, to run only the backend:

```dotenv
COMPOSE_PROFILES=backend
```

### Activating profiles via CLI flags

You can also pass profiles directly on the command line:

```sh
docker compose --profile backend --profile auth-ui up -d
```

Ports and dev credentials use sensible defaults baked into the compose file; override them in `.env` only if you have a port conflict or want different DB credentials. See the `${VAR:-default}` references in `docker-compose.yml` for the full list.
