import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";
import {
  getShellOutput,
  getShellStatistics,
  sendShellCommand,
} from "../components/Api";
import dayjs from "dayjs";
import duration from "dayjs/plugin/duration";
import relativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(duration);
dayjs.extend(relativeTime);

export default function InteractiveShell() {
  const { uuid } = useParams<{ uuid: string }>();
  const [command, setCommand] = useState("");
  const [runningFrame, setRunningFrame] = useState(0);
  const [isWindowFocused, setIsWindowFocused] = useState(() =>
    typeof document !== "undefined" ? document.hasFocus() : true,
  );
  const outputContainerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const handleFocus = () => setIsWindowFocused(true);
    const handleBlur = () => setIsWindowFocused(false);

    window.addEventListener("focus", handleFocus);
    window.addEventListener("blur", handleBlur);

    return () => {
      window.removeEventListener("focus", handleFocus);
      window.removeEventListener("blur", handleBlur);
    };
  }, []);

  const {
    data: terminalOutput,
    isLoading: isOutputLoading,
    error: outputError,
    refetch: refetchOutput,
  } = useQuery({
    queryKey: ["shellOutput", uuid],
    queryFn: () => {
      if (!uuid) {
        throw new Error("Shell session UUID is missing in route.");
      }
      return getShellOutput(uuid);
    },
    enabled: Boolean(uuid) && isWindowFocused,
    refetchInterval: 1500,
  });

  const {
    data: agentStatistics,
    isLoading: isStatisticsLoading,
    error: statisticsError,
  } = useQuery({
    queryKey: ["shellStatistics", uuid],
    queryFn: () => {
      if (!uuid) {
        throw new Error("Shell session UUID is missing in route.");
      }
      return getShellStatistics(uuid);
    },
    enabled: Boolean(uuid) && isWindowFocused,
    refetchInterval: 5000,
  });

  const runCommandMutation = useMutation({
    mutationFn: (cmd: string) => {
      if (!uuid) {
        throw new Error("Shell session UUID is missing in route.");
      }
      return sendShellCommand(uuid, cmd);
    },
    onSuccess: () => {
      void refetchOutput();
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cmd = command.trim();
    if (!cmd || runCommandMutation.isPending) {
      return;
    }
    setCommand("");
    runCommandMutation.mutate(cmd);
  }

  useEffect(() => {
    if (!runCommandMutation.isPending) {
      setRunningFrame(0);
      return;
    }
    const intervalId = window.setInterval(() => {
      setRunningFrame((previous) => (previous + 1) % 3);
    }, 450);
    return () => window.clearInterval(intervalId);
  }, [runCommandMutation.isPending]);

  useEffect(() => {
    const outputContainer = outputContainerRef.current;
    if (!outputContainer) {
      return;
    }
    outputContainer.scrollTop = outputContainer.scrollHeight;
  }, [terminalOutput, runCommandMutation.isPending]);

  if (!uuid) {
    return (
      <div className="container mt-4">
        <div className="alert alert-danger mb-0">
          Invalid route. Use /shell/&lt;uuid&gt;.
        </div>
      </div>
    );
  }

  return (
    <div className="container mt-4">
      <h1>Interactive Shell</h1>
      <p className="text-muted mb-3">Shell session: {uuid}</p>

      <div className="card">
        <div
          ref={outputContainerRef}
          className="card-body bg-dark text-light"
          style={{
            minHeight: "360px",
            maxHeight: "420px",
            overflowY: "auto",
            fontFamily: "monospace",
            whiteSpace: "pre-wrap",
          }}
        >
          {isOutputLoading && !terminalOutput ? (
            <p className="text-secondary mb-0">Loading terminal output...</p>
          ) : outputError && !terminalOutput ? (
            <p className="text-danger mb-0">
              Failed to fetch output: {(outputError as Error).message}
            </p>
          ) : terminalOutput ? (
            <pre className="mb-0 text-light">{terminalOutput}</pre>
          ) : (
            <p className="text-secondary mb-0">
              Enter a command to start interacting with this shell session.
            </p>
          )}
          {runCommandMutation.isPending ? (
            <div className="text-secondary mt-2">
              Running{".".repeat(runningFrame + 1)}
            </div>
          ) : null}
        </div>
      </div>

      <form className="mt-3" onSubmit={handleSubmit}>
        <label className="form-label" htmlFor="shell-command">
          Command
        </label>
        <div className="input-group">
          <input
            id="shell-command"
            className="form-control"
            value={command}
            onChange={(event) => setCommand(event.target.value)}
            placeholder="uname -a"
            autoComplete="off"
          />
          <button
            type="submit"
            className="btn btn-primary"
            disabled={runCommandMutation.isPending || !command.trim()}
          >
            {runCommandMutation.isPending ? "Running..." : "Run"}
          </button>
        </div>
      </form>

      {runCommandMutation.error ? (
        <div className="alert alert-danger mt-3 mb-0">
          Failed to send command: {(runCommandMutation.error as Error).message}
        </div>
      ) : null}

      <div className="card mt-4">
        <div className="card-header">Agent Statistics</div>
        <div className="card-body">
          {isStatisticsLoading ? (
            <p className="mb-0 text-muted">Loading statistics...</p>
          ) : statisticsError ? (
            <p className="mb-0 text-danger">
              Failed to fetch statistics: {(statisticsError as Error).message}
            </p>
          ) : agentStatistics ? (
            <div className="row g-3">
              <div className="col-sm-6 col-lg-3">
                <div className="border rounded p-2">
                  <div className="text-muted small">Uptime</div>
                  <div>
                    {dayjs
                      .duration(agentStatistics.uptime_seconds, "seconds")
                      .humanize()}
                  </div>
                </div>
              </div>
              <div className="col-sm-6 col-lg-3">
                <div className="border rounded p-2">
                  <div className="text-muted small">CPU Usage</div>
                  <div>{agentStatistics.cpu_usage}</div>
                </div>
              </div>
              <div className="col-sm-6 col-lg-3">
                <div className="border rounded p-2">
                  <div className="text-muted small">RAM Usage</div>
                  <div>{agentStatistics.ram_usage}</div>
                </div>
              </div>
              <div className="col-sm-6 col-lg-3">
                <div className="border rounded p-2">
                  <div className="text-muted small">Disk Usage</div>
                  <div>{agentStatistics.disk_usage}</div>
                </div>
              </div>
            </div>
          ) : (
            <p className="mb-0 text-muted">No statistics available.</p>
          )}
        </div>
      </div>
    </div>
  );
}
