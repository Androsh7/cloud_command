package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/androsh7/cloud_command/internal/control"
	"github.com/androsh7/cloud_command/internal/mcpserver"
	"github.com/androsh7/cloud_command/pkg/history"
)

const (
	flagJson         = "json"
	flagDetached     = "detached"
	flagCount        = "count"
	flagRegion       = "region"
	flagInstanceType = "instance-type"
	flagArchitecture = "architecture"
	flagNoWait       = "no-wait"
	flagYes          = "yes"
	flagSudo         = "sudo"
	flagLines        = "lines"
	timestampLayout  = "2006-01-02 15:04:05"
)

var errRefused = errors.New("refused")

// newMcpCommand serves the MCP tools over standard input and output.
func newMcpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Serve the agent tools to an MCP client over stdio",
		Long:  "Serve the agent tools to an MCP client over stdio. Nothing but protocol traffic is written to stdout; logs go to the log file.",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				controller := control.New(app.store, app.settings, app.history, history.SourceMcp)
				return mcpserver.ServeStdio(ctx, mcpserver.New(controller))
			})
		},
	}
}

// newAgentsCommand groups the agent subcommands.
func newAgentsCommand() *cobra.Command {
	agents := &cobra.Command{Use: "agents", Short: "List, create and terminate agents"}
	agents.AddCommand(newAgentsListCommand(), newAgentsCreateCommand(), newAgentsTerminateCommand())
	return agents
}

// newAgentsListCommand lists the stored agents.
func newAgentsListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "list",
		Short: "List agents with their instance state",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			asJson, _ := command.Flags().GetBool(flagJson)
			includeDetached, _ := command.Flags().GetBool(flagDetached)
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				agents, detached, err := app.controller.ListAgents(ctx, includeDetached)
				if err != nil {
					return err
				}
				if asJson {
					return printJson(command.OutOrStdout(), map[string]any{"agents": agents, "detached": detached})
				}
				writer := newTable(command.OutOrStdout())
				fmt.Fprintln(writer, "NAME\tSTATE\tREGION\tTYPE\tPUBLIC IP\tINSTANCE\tJOBS")
				for _, summary := range agents {
					fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%d\n", summary.Name, summary.State, summary.Region, summary.InstanceType, summary.PublicIp, summary.InstanceId, summary.JobCount)
				}
				if len(detached) > 0 {
					fmt.Fprintln(writer, "\nDETACHED INSTANCE\tSTATE\tREGION\tKEY PAIR")
					for _, instance := range detached {
						fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", instance.InstanceId, instance.State, instance.Region, instance.KeyPair)
					}
				}
				return writer.Flush()
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	command.Flags().Bool(flagDetached, false, "also scan every region for tagged instances without a local record")
	return command
}

// newAgentsCreateCommand creates one agent or a numbered batch.
func newAgentsCreateCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "create <name>",
		Short: "Create an agent, or a batch named <name>-1..N with --count",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			flags := command.Flags()
			asJson, _ := flags.GetBool(flagJson)
			count, _ := flags.GetInt(flagCount)
			region, _ := flags.GetString(flagRegion)
			instanceType, _ := flags.GetString(flagInstanceType)
			architecture, _ := flags.GetString(flagArchitecture)
			noWait, _ := flags.GetBool(flagNoWait)
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				results, err := app.controller.CreateAgents(ctx, control.CreateRequest{
					Name: args[0], Count: count, Region: region, InstanceType: instanceType, Architecture: architecture, Wait: !noWait,
				})
				if err != nil {
					return err
				}
				if asJson {
					return printJson(command.OutOrStdout(), results)
				}
				writer := newTable(command.OutOrStdout())
				fmt.Fprintln(writer, "NAME\tINSTANCE\tSTATE\tPUBLIC IP\tERROR")
				failed := false
				for _, result := range results {
					failed = failed || result.Error != ""
					fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", result.Name, result.InstanceId, result.State, result.PublicIp, result.Error)
				}
				if err := writer.Flush(); err != nil {
					return err
				}
				if failed {
					return errors.New("one or more agents failed to create")
				}
				return nil
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	command.Flags().Int(flagCount, 1, "number of agents to create")
	command.Flags().String(flagRegion, "", "AWS region (default from settings)")
	command.Flags().String(flagInstanceType, "", "EC2 instance type (default from settings)")
	command.Flags().String(flagArchitecture, "", "arm64 or x86_64 (default from settings)")
	command.Flags().Bool(flagNoWait, false, "return after launch instead of waiting for SSH")
	return command
}

// newAgentsTerminateCommand destroys agents.
func newAgentsTerminateCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "terminate <name>...",
		Short: "Terminate agents and delete their key pairs and local records",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			asJson, _ := command.Flags().GetBool(flagJson)
			confirmed, _ := command.Flags().GetBool(flagYes)
			if !confirmed {
				return fmt.Errorf("%w: pass --%s to terminate %s", errRefused, flagYes, strings.Join(args, ", "))
			}
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				results := app.controller.TerminateAgents(ctx, args)
				if asJson {
					return printJson(command.OutOrStdout(), results)
				}
				failed := false
				for _, result := range results {
					if result.Error != "" {
						failed = true
						fmt.Fprintf(command.OutOrStdout(), "%s: %s\n", result.Name, result.Error)
					} else {
						fmt.Fprintf(command.OutOrStdout(), "%s: terminated\n", result.Name)
					}
				}
				if failed {
					return errors.New("one or more agents failed to terminate")
				}
				return nil
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	command.Flags().Bool(flagYes, false, "confirm the termination")
	return command
}

// newJobsCommand groups the job subcommands.
func newJobsCommand() *cobra.Command {
	jobs := &cobra.Command{Use: "jobs", Short: "Start, inspect, stop and remove persistent jobs"}
	jobs.AddCommand(newJobsListCommand(), newJobsStartCommand(), newJobsStatusCommand(), newJobsStopCommand(), newJobsRemoveCommand())
	return jobs
}

// newJobsListCommand lists the jobs of one agent.
func newJobsListCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "list <agent>",
		Short: "List an agent's jobs with their state",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			asJson, _ := command.Flags().GetBool(flagJson)
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				detail, err := app.controller.GetAgent(ctx, args[0])
				if err != nil {
					return err
				}
				if asJson {
					return printJson(command.OutOrStdout(), detail.Jobs)
				}
				writer := newTable(command.OutOrStdout())
				fmt.Fprintln(writer, "JOB\tSTATUS\tSTARTED\tSUDO\tSCRIPT")
				for _, job := range detail.Jobs {
					fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%s\n", job.Id, job.Status, job.StartedAt.Local().Format(timestampLayout), job.Sudo, firstLine(job.Script))
				}
				return writer.Flush()
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	return command
}

// newJobsStartCommand starts a persistent job.
func newJobsStartCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "start <agent> -- <script>",
		Short: "Start a script as a persistent systemd job",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			asJson, _ := command.Flags().GetBool(flagJson)
			sudo, _ := command.Flags().GetBool(flagSudo)
			script := strings.Join(args[1:], " ")
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				job, err := app.controller.StartJob(ctx, args[0], script, sudo)
				if err != nil {
					return err
				}
				if asJson {
					return printJson(command.OutOrStdout(), job)
				}
				fmt.Fprintln(command.OutOrStdout(), job.Id)
				return nil
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	command.Flags().Bool(flagSudo, false, "run the job as root")
	return command
}

// newJobsStatusCommand shows a job's state and output.
func newJobsStatusCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "status <agent> <job-id>",
		Short: "Show a job's state, exit code and trailing output",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			asJson, _ := command.Flags().GetBool(flagJson)
			lines, _ := command.Flags().GetInt(flagLines)
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				output, err := app.controller.JobStatus(ctx, args[0], args[1], lines)
				if err != nil {
					return err
				}
				if asJson {
					return printJson(command.OutOrStdout(), output)
				}
				out := command.OutOrStdout()
				fmt.Fprintf(out, "job %s on %s: %s\n", output.JobId, output.Agent, output.Status)
				if output.Stdout != "" {
					fmt.Fprintf(out, "--- stdout ---\n%s\n", output.Stdout)
				}
				if output.Stderr != "" {
					fmt.Fprintf(out, "--- stderr ---\n%s\n", output.Stderr)
				}
				return nil
			})
		},
	}
	command.Flags().Bool(flagJson, false, "print JSON")
	command.Flags().Int(flagLines, control.DefaultOutputLines, "trailing output lines to show")
	return command
}

// newJobsStopCommand stops a job.
func newJobsStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <agent> <job-id>",
		Short: "Stop a running job, keeping its record and output",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				return app.controller.StopJob(ctx, args[0], args[1])
			})
		},
	}
}

// newJobsRemoveCommand stops and forgets a job.
func newJobsRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <agent> <job-id>",
		Short: "Stop a job, delete its output files and forget it",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			return withApplication(command.Context(), func(ctx context.Context, app *application) error {
				return app.controller.RemoveJob(ctx, args[0], args[1])
			})
		},
	}
}

// printJson writes a value as indented JSON.
func printJson(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// newTable returns a tab-aligned writer for human output.
func newTable(writer io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
}

// firstLine returns the first line of a script for compact listings.
func firstLine(script string) string {
	line, _, _ := strings.Cut(script, "\n")
	return line
}
