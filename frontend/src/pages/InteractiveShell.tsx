import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";
import { executeCommand, statusAgent } from "../components/Api";
import dayjs from "dayjs";
import duration from "dayjs/plugin/duration";
import relativeTime from "dayjs/plugin/relativeTime";

dayjs.extend(duration);
dayjs.extend(relativeTime);

type ShellEntry = {
  id: number;
  command: string;
  stdout: string;
  stderr: string;
  exitCode: number | null;
  isPending: boolean;
  error?: string;
};

export default function InteractiveShell() {
  const { agentName } = useParams<{ agentName: string }>();
  const [command, setCommand] = useState("");
  const [history, setHistory] = useState<ShellEntry[]>([]);
  const [runningFrame, setRunningFrame] = useState(0);
  const [isWindowFocused, setIsWindowFocused] = useState(() =>
    typeof document !== "undefined" ? document.hasFocus() : true,
  );
  const outputContainerRef = useRef<HTMLDivElement | null>(null);
  const nextEntryIdRef = useRef(1);

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
    data: agentStatistics,
    isLoading: isStatisticsLoading,
    error: statisticsError,
  } = useQuery({
    queryKey: ["agentStatistics", agentName],
    queryFn: () => {
      if (!agentName) {
        throw new Error("Agent name is missing in route.");
      }
      return statusAgent(agentName);
    },
    enabled: Boolean(agentName) && isWindowFocused,
    refetchInterval: 5000,
  });

  const runCommandMutation = useMutation({
    mutationFn: ({ cmd }: { cmd: string; entryId: number }) => {
      if (!agentName) {
        throw new Error("Agent name is missing in route.");
      }
      return executeCommand(agentName, cmd);
    },
    onSuccess: (result, { entryId }) => {
      setHistory((previous) =>
        previous.map((entry) =>
          entry.id === entryId
            ? {
                ...entry,
                stdout: result.stdout,
                stderr: result.stderr,
                exitCode: result.exit_code,
                isPending: false,
              }
            : entry,
        ),
      );
    },
    onError: (error, { entryId }) => {
      setHistory((previous) =>
        previous.map((entry) =>
          entry.id === entryId
            ? {
                ...entry,
                stdout: "",
                stderr: "",
                exitCode: null,
                isPending: false,
                error: (error as Error).message,
              }
            : entry,
        ),
      );
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cmd = command.trim();
    if (!cmd || runCommandMutation.isPending) {
      return;
    }
    const entryId = nextEntryIdRef.current++;
    setHistory((previous) => [
      ...previous,
      {
        id: entryId,
        command: cmd,
        stdout: "",
        stderr: "",
        exitCode: null,
        isPending: true,
      },
    ]);
    setCommand("");
    runCommandMutation.mutate({ cmd, entryId });
  }

  useEffect(() => {
    const hasPendingEntry = history.some((entry) => entry.isPending);
    if (!hasPendingEntry) {
      setRunningFrame(0);
      return;
    }
    const intervalId = window.setInterval(() => {
      setRunningFrame((previous) => (previous + 1) % 3);
    }, 450);
    return () => window.clearInterval(intervalId);
  }, [history]);

  useEffect(() => {
    const outputContainer = outputContainerRef.current;
    if (!outputContainer) {
      return;
    }
    outputContainer.scrollTop = outputContainer.scrollHeight;
  }, [history]);

  if (!agentName) {
    return (
      <div className="container mt-4">
        <div className="alert alert-danger mb-0">
          Invalid route. Use /shell/&lt;agent-name&gt;.
        </div>
      </div>
    );
  }

  return (
    <div className="container mt-4">
      <h1>Interactive Shell</h1>
      <p className="text-muted mb-3">Connected to agent: {agentName}</p>

      <div className="card">
        <div
          ref={outputContainerRef}
          className="card-body bg-dark text-light"
          style={{
            minHeight: "360px",
            maxHeight: "420px",
            overflowY: "auto",
            fontFamily: "monospace",
          }}
        >
          {history.length === 0 ? (
            <p className="text-secondary mb-0">
              Enter a command to run it on {agentName}.
            </p>
          ) : (
            history.map((entry) => (
              <div key={entry.id} className="mb-3">
                <div>
                  <span className="text-info">{agentName}</span>:~${" "}
                  {entry.command}
                </div>
                {entry.isPending ? (
                  <div className="text-secondary">
                    Running{".".repeat(runningFrame + 1)}
                  </div>
                ) : null}
                {entry.stdout ? (
                  <pre className="mb-1 text-light">{entry.stdout}</pre>
                ) : null}
                {entry.stderr ? (
                  <pre className="mb-1 text-warning">{entry.stderr}</pre>
                ) : null}
                {entry.error ? (
                  <div className="text-danger">
                    Request failed: {entry.error}
                  </div>
                ) : null}
                {entry.exitCode !== null ? (
                  <small className="text-secondary">
                    Exit code: {entry.exitCode}
                  </small>
                ) : null}
              </div>
            ))
          )}
        </div>
      </div>

      <form className="mt-3" onSubmit={handleSubmit}>
        <label className="form-label" htmlFor="shell-command">
          Command
        </label>
        <div className="input-group">
          <span className="input-group-text">{agentName}:~$</span>
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
