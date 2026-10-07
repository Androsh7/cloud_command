# cloud-command

A terminal interface for running work on disposable AWS EC2 agents. It creates agents (one or a batch), opens persistent bash shells on one or many of them over SSH, starts persistent commands as `systemd-run` units, and tears everything down again.

## Requirements

- Go 1.27 or newer.
- AWS credentials available to the default SDK credential chain (`~/.aws/credentials`, `AWS_PROFILE`, environment variables or SSO).
- Agents run Amazon Linux 2023, which provides `systemd-run` for persistent jobs.

## Run

```bash
make run
```

`make build` stamps the version from `VERSION.txt` into the title, for example `Cloud Command v1.0.0`. `make build-dev` and `make run` show `Cloud Command DEVELOPMENT`.

The first run writes `~/.cloud-command/settings.toml` with defaults and opens the agent list.

## Files under `~/.cloud-command`

```
settings.toml                   user defaults (see below)
history.toml                    every command and job ever sent, from the TUI, the CLI and the MCP server
cloud-command.log               JSON log
agents/<name>/agent.toml        agent record: ids, public ip, key file names, jobs
agents/<name>/private_rsa.key   private key (0600)
agents/<name>/public_rsa.key    public key imported to EC2 as ssh-key-<uuid>
```

### settings.toml

| Key | Default | Meaning |
| --- | --- | --- |
| `region` | `us-east-1` | Region prefilled in the create form |
| `instance_type` | `t4g.nano` | Instance type prefilled in the create form |
| `architecture` | `arm64` | `arm64` or `x86_64`; selects the Amazon Linux 2023 AMI |
| `ssh_user` | `ec2-user` | Login user on the agent |
| `ssh_port` | `22` | SSH port |
| `ssh_timeout` | `10s` | Dial timeout per connection |
| `ready_timeout` | `5m` | Maximum wait for a new agent to run and accept SSH |
| `poll_interval` | `5s` | SSH retry interval and job status refresh interval |
| `state_refresh_interval` | `15s` | How often the agent list re-reads instance states from AWS |
| `region_search_threads` | `0` | Regions scanned at once for detached instances; `0` means all |
| `log_level` | `info` | `debug`, `info`, `warn` or `error` |

## Key bindings

| Screen | Keys |
| --- | --- |
| Agents | `enter` detail, `n` new agent, `b` batch, `space` select, `a` select all, `c` shell on selection, `t` terminate selection, `h` history, `r` refresh, `q` quit |
| Create form | `tab` / `shift+tab` move between fields, `enter` create, `esc` back |
| Creating | `esc` cancel, `enter` back when finished |
| Detail | `c` shell, `J` start persistent job, `j` jobs, `t` terminate, `r` refresh, `esc` back |
| Shell | `enter` send the line to every host, `up`/`down` history, `pgup`/`pgdn` scroll, `tab` switch between all hosts and one host at a time, `ctrl+o` cycle stdout+stderr / stdout / stderr, `ctrl+x` interrupt (Ctrl-C), `ctrl+l` clear, `ctrl+s` save transcripts, `esc` back (shells stay open), `ctrl+q` close the shells and go back |
| Job editor | `ctrl+r` start job, `ctrl+s` toggle sudo, `esc` back |
| Jobs | `enter` detail, `s` stop, `d` stop and remove, `r` refresh, `esc` back |
| Job detail | `o` switch stdout/stderr, `s` stop, `r` refresh, `esc` back |
| History | `enter` full entry, `r` refresh, `esc` back |

`ctrl+c` quits from any screen.

The agents screen also lists instances tagged `Type=cloud-command` that have no local record. They can be terminated with `t`.

## Shells

`c` opens a persistent `/bin/bash` on each selected agent under a pseudo-terminal. Working directory, variables and exports survive between commands, and the shell stays open when you leave the screen. Stdout and stderr stream into the view as they arrive; when several hosts are open each line is prefixed with `[host]`. Every line you type is sent to every host shown; press `tab` to focus one host and the input, interrupt and view apply to that host alone. Stderr lines are shown in red and `ctrl+o` filters the view to stdout or stderr only. Each finished command adds an `[exit N]` line to the transcript (bash reports it through its prompt hook, so programs that read the terminal cannot swallow it), and the per-host status shows `running` or `idle (exit N)`. `ctrl+x` sends Ctrl-C to the foreground command on every host. If a shell exits (for example after `exit`), the next line you send reopens it.

`ctrl+s` saves the transcripts under `~/.cloud-command/output/`: one host is saved as `<timestamp>-<agent>.txt`, several as a `<timestamp>/` directory with one `<agent>.txt` per host.

## History

Every shell line, `run_command` call and job start is recorded in `~/.cloud-command/history.toml` with its time, agent, source (`tui`, `cli` or `mcp`), sudo flag, full script, job id and exit code once known. The record outlives the agent. Press `h` on the agent list to browse it newest first and `enter` to read the full script of an entry. The job detail screen also shows a job's full script.

## Persistent jobs

A job runs as a transient unit named `job-<uuid>` with `RemainAfterExit=yes`, so its exit code stays visible until the job is removed. Standard output and standard error are written to `/tmp/job-<uuid>.stdout` and `/tmp/job-<uuid>.stderr` on the agent. Stopping a job runs `systemctl stop`; removing it also resets the unit and deletes the output files.

## Command line

Every operation the TUI offers for creating, listing and terminating agents and for jobs is also a subcommand, for scripts and CI. Add `--json` to get the raw result instead of a table.

```bash
cloud-command agents list [--detached] [--json]
cloud-command agents create <name> [--count N] [--region R] [--instance-type T] [--architecture arm64|x86_64] [--no-wait] [--json]
cloud-command agents terminate <name>... --yes
cloud-command jobs list <agent> [--json]
cloud-command jobs start <agent> [--sudo] -- '<script>'
cloud-command jobs status <agent> <job-id> [--lines N] [--json]
cloud-command jobs stop <agent> <job-id>
cloud-command jobs remove <agent> <job-id>
```

`agents create` waits until the agents accept SSH unless `--no-wait` is given, in which case it returns right after launch and `agents list` shows the state. `agents terminate` refuses to run without `--yes`. Job ids may be shortened to any unambiguous prefix. Errors go to stderr and the exit code is 1 when anything failed.

## MCP server

`cloud-command mcp` serves the same operations to an MCP client over stdio. Claude Code picks it up from the `.mcp.json` in this repository; for another project or for Claude Desktop add:

```json
{
  "mcpServers": {
    "cloud-command": {
      "type": "stdio",
      "command": "cloud-command",
      "args": ["mcp"]
    }
  }
}
```

`cloud-command` must be on `PATH`, or use the absolute path to the binary. The server reads settings and agents from `~/.cloud-command` like the TUI does, and its instructions tell the model to reuse agents, keep instances small and never terminate without permission.

| Tool | Purpose |
| --- | --- |
| `list_agents` | Agents with instance state, public IP and job count; `include_detached` also scans every region |
| `get_agent` | One agent with its ssh command and the state of each job |
| `create_agent` | One agent; `wait: false` returns after launch |
| `terminate_agent` | Terminate one agent, delete its key pair and record |
| `run_command` | Run a bash script on one agent and return exit code, stdout and stderr |
| `start_job` | Start a persistent systemd job and return its id |
| `job_status` | State, exit code and trailing output of a job |
| `stop_job` | Stop a job, keeping its record |
| `remove_job` | Stop a job, delete its output and forget it |

In `mcp` mode stdout carries the protocol, so nothing else is written there; logs go to `~/.cloud-command/cloud-command.log`. The TUI, the command line and the MCP server may run at the same time against the same directory. Each operation reloads the agent record before changing it and writes it atomically, but two processes changing the same agent's jobs in the same instant can still lose one record.

## Tests

```bash
make test                 # unit tests, no network
make sshd-up              # local sshd on 127.0.0.1:2222 (needs ~/.ssh/sshd_container and .pub)
make test-live-ssh        # SSH client tests against the container
make sshd-down
make test-live-aws        # creates, exercises and destroys one real EC2 agent
```

The live SSH tests read `CLOUD_COMMAND_SSH_HOST`, `CLOUD_COMMAND_SSH_PORT`, `CLOUD_COMMAND_SSH_USER` and `CLOUD_COMMAND_SSH_KEY` to target another host.
