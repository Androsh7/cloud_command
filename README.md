# cloud_command

A terminal interface for running work on disposable AWS EC2 agents. It creates agents (one or a batch), runs shell commands on one or many of them over SSH, starts persistent commands as `systemd-run` units, and tears everything down again.

## Requirements

- Go 1.27 or newer.
- AWS credentials available to the default SDK credential chain (`~/.aws/credentials`, `AWS_PROFILE`, environment variables or SSO).
- Agents run Amazon Linux 2023, which provides `systemd-run` for persistent jobs.

## Run

```bash
make build
./cloud_command
```

The first run writes `~/.cloud_command/settings.toml` with defaults and opens the agent list.

## Files under `~/.cloud_command`

```
settings.toml                   user defaults (see below)
cloud_command.log               JSON log
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
| `region_search_threads` | `0` | Regions scanned at once for detached instances; `0` means all |
| `log_level` | `info` | `debug`, `info`, `warn` or `error` |

## Key bindings

| Screen | Keys |
| --- | --- |
| Agents | `enter` detail, `n` new agent, `b` batch, `space` select, `a` select all, `c` run command on selection, `t` terminate, `r` refresh, `q` quit |
| Create form | `tab` / `shift+tab` move between fields, `enter` create, `esc` back |
| Creating | `esc` cancel, `enter` back when finished |
| Detail | `c` run command, `J` start persistent job, `j` jobs, `t` terminate, `r` refresh, `esc` back |
| Command editor | `ctrl+r` run, `ctrl+s` toggle sudo, `esc` back |
| Output | arrow keys scroll, `esc` back |
| Jobs | `enter` detail, `s` stop, `d` stop and remove, `r` refresh, `esc` back |
| Job detail | `o` switch stdout/stderr, `s` stop, `r` refresh, `esc` back |

`ctrl+c` quits from any screen.

The agents screen also lists instances tagged `Type=cloud_command` that have no local record. They can be terminated with `t`.

## Persistent jobs

A job runs as a transient unit named `job-<uuid>` with `RemainAfterExit=yes`, so its exit code stays visible until the job is removed. Standard output and standard error are written to `/tmp/job-<uuid>.stdout` and `/tmp/job-<uuid>.stderr` on the agent. Stopping a job runs `systemctl stop`; removing it also resets the unit and deletes the output files.

## Tests

```bash
make test                 # unit tests, no network
make sshd-up              # local sshd on 127.0.0.1:2222 (needs ~/.ssh/sshd_container and .pub)
make test-live-ssh        # SSH client tests against the container
make sshd-down
make test-live-aws        # creates, exercises and destroys one real EC2 agent
```

The live SSH tests read `CLOUD_COMMAND_SSH_HOST`, `CLOUD_COMMAND_SSH_PORT`, `CLOUD_COMMAND_SSH_USER` and `CLOUD_COMMAND_SSH_KEY` to target another host.
