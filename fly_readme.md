# Fly.io Notes

This repo can run on Fly as a single worker-style machine, but there is one important detail:

- do not run the default container command on Fly
- do not use `start`
- run `__run`

Why:

- the Docker image defaults to `help`
- `start` is for local detached background mode
- `__run` keeps the process in the foreground, which is what Fly expects
- in [`main.go`](/Users/aa/os/dialtone-watcher/main.go), `__run` also forces `TestMode=true`

## Current Working App

- Fly app: `proud-glade-2163`
- Region: `sjc`
- Machine count: `1`
- Machine ID from this deploy: `8d31e4fe232508`
- Current VM memory target: `2 GB`

## Files

- [`Dockerfile`](/Users/aa/os/dialtone-watcher/Dockerfile)
- [`fly.toml`](/Users/aa/os/dialtone-watcher/fly.toml)
- [`linux.md`](/Users/aa/os/dialtone-watcher/linux.md)

## Important Config

This is the key part of `fly.toml`:

```toml
app = "proud-glade-2163"
primary_region = "sjc"
kill_signal = "SIGTERM"

[processes]
  app = "__run"
```

Notes:

- `kill_signal = "SIGTERM"` matches the watcher shutdown handling
- there is no `http_service` block
- this app is a worker, not a web server
- if Fly generates an `http_service` section during `fly launch`, remove it before deploy

## Launch A New Fly App

From the repo root:

```bash
fly launch \
  --generate-name \
  --copy-config \
  --command "__run" \
  --ha=false \
  --no-public-ips \
  --no-github-workflow \
  --no-db \
  --no-object-storage \
  --no-redis \
  --no-deploy \
  --yes \
  --region sjc
```

Then inspect `fly.toml` and remove any generated `http_service` block if Fly added one.

## Deploy

Use a single-machine deploy from the repo root:

```bash
fly deploy --local-only --ha=false --strategy immediate --yes -a proud-glade-2163
```

What this does:

- builds the image from the local `Dockerfile`
- pushes it to Fly registry
- creates one machine
- runs the binary with `__run`

## Verify

Check app status:

```bash
fly status -a proud-glade-2163
```

List machines:

```bash
fly machine list -a proud-glade-2163
```

Check one machine in detail:

```bash
fly machine status 8d31e4fe232508 -a proud-glade-2163
```

Read logs:

```bash
fly logs -a proud-glade-2163 --no-tail
```

What you want to see:

- one machine in `started` state
- Fly starting `/app/dialtone-watcher __run`

Typical startup log line:

```text
Preparing to run: `/app/dialtone-watcher __run`
```

## Why `__run` Instead Of `start`

Local CLI behavior:

```bash
./dialtone-watcher start
./dialtone-watcher stop
./dialtone-watcher summary
```

That is fine on a laptop or VM you manage directly.

On Fly:

- `start` forks into detached mode
- the parent process exits
- Fly wants the main process to stay in the foreground
- `__run` is the correct process model for a container worker

## Test Mode Behavior

`__run` enables test mode in the current code path.

Relevant behavior from the repo:

- upload interval defaults to `15s` in test mode
- Linux test traffic generation is enabled in test mode
- Linux test mode now creates mixed HTTPS, HTTP, DNS, UDP DNS, and short CPU bursts

That makes Fly useful for Linux-side smoke testing instead of only printing `help`.

## If You Want A Different App Name

Replace the app name in commands:

```bash
fly status -a YOUR_APP_NAME
fly machine list -a YOUR_APP_NAME
fly deploy --local-only --ha=false --strategy immediate --yes -a YOUR_APP_NAME
```

Also update `app = "YOUR_APP_NAME"` in `fly.toml`.

## Common Mistakes

- leaving the Docker default command alone, which only prints `help`
- using `start` instead of `__run`
- keeping a generated `http_service` block for a non-HTTP binary
- assuming a started machine means the app is doing useful work without checking logs

## Minimal Repeatable Flow

```bash
fly launch --generate-name --copy-config --command "__run" --ha=false --no-public-ips --no-github-workflow --no-db --no-object-storage --no-redis --no-deploy --yes --region sjc
fly deploy --local-only --ha=false --strategy immediate --yes -a proud-glade-2163
fly machine list -a proud-glade-2163
fly logs -a proud-glade-2163 --no-tail
```
