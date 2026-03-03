import { useMutation, useQuery } from "@tanstack/react-query";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
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
  const bottomAnchorRef = useRef<HTMLDivElement | null>(null);
  const bottomStripRef = useRef<HTMLDivElement | null>(null);
  const [bottomStripHeight, setBottomStripHeight] = useState(0);

  const scrollOutputToBottom = useCallback(() => {
    const outputContainer = outputContainerRef.current;
    const bottomAnchor = bottomAnchorRef.current;
    if (!outputContainer) {
      return;
    }

    outputContainer.scrollTop = outputContainer.scrollHeight;
    bottomAnchor?.scrollIntoView({ block: "end" });
  }, []);

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
    refetchInterval: 500,
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
      window.setTimeout(() => void refetchOutput(), 120);
      window.setTimeout(() => void refetchOutput(), 300);
      scrollOutputToBottom();
    },
  });

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cmd = command.trim();
    if (!cmd || runCommandMutation.isPending) {
      return;
    }
    setCommand("");
    scrollOutputToBottom();
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

  useLayoutEffect(() => {
    scrollOutputToBottom();
  }, [terminalOutput, runCommandMutation.isPending, scrollOutputToBottom]);

  useEffect(() => {
    const outputContainer = outputContainerRef.current;
    if (!outputContainer) {
      return;
    }
    const observer = new MutationObserver(() => {
      scrollOutputToBottom();
    });
    observer.observe(outputContainer, {
      childList: true,
      subtree: true,
      characterData: true,
    });

    return () => {
      observer.disconnect();
    };
  }, [scrollOutputToBottom]);

  useLayoutEffect(() => {
    const bottomStrip = bottomStripRef.current;
    if (!bottomStrip) {
      return;
    }

    const syncHeight = () => {
      setBottomStripHeight(bottomStrip.offsetHeight);
    };

    syncHeight();
    const resizeObserver = new ResizeObserver(syncHeight);
    resizeObserver.observe(bottomStrip);

    return () => {
      resizeObserver.disconnect();
    };
  }, []);

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
    <div className="shell-page">
      <div
        ref={outputContainerRef}
        className="shell-output"
        style={{ paddingBottom: `${bottomStripHeight + 24}px` }}
      >
        {isOutputLoading && !terminalOutput ? (
          <p className="text-secondary mb-0">Loading terminal output...</p>
        ) : outputError && !terminalOutput ? (
          <p className="text-danger mb-0">
            Failed to fetch output: {(outputError as Error).message}
          </p>
        ) : terminalOutput ? (
          <pre className="shell-output-pre">{terminalOutput}</pre>
        ) : (
          <p className="text-secondary mb-0">
            Enter a command to start interacting with this shell session.
          </p>
        )}
        <div ref={bottomAnchorRef} />
      </div>

      <div ref={bottomStripRef} className="shell-bottom-strip">
        <div className="shell-session-label">
          Session: <code>{uuid}</code>
        </div>

        <div className="shell-stats-row">
          {isStatisticsLoading ? (
            <span className="badge text-bg-secondary">Loading stats...</span>
          ) : statisticsError ? (
            <span className="badge text-bg-danger">
              Stats unavailable: {(statisticsError as Error).message}
            </span>
          ) : agentStatistics ? (
            <>
              <span className="badge text-bg-secondary">
                Uptime:{" "}
                {dayjs
                  .duration(agentStatistics.uptime_seconds, "seconds")
                  .humanize()}
              </span>
              <span className="badge text-bg-secondary">
                CPU: {agentStatistics.cpu_usage}
              </span>
              <span className="badge text-bg-secondary">
                RAM: {agentStatistics.ram_usage}
              </span>
              <span className="badge text-bg-secondary">
                Disk: {agentStatistics.disk_usage}
              </span>
            </>
          ) : (
            <span className="badge text-bg-secondary">No stats available</span>
          )}

          {runCommandMutation.isPending ? (
            <span className="badge text-bg-info">
              Running{".".repeat(runningFrame + 1)}
            </span>
          ) : null}
        </div>

        <form onSubmit={handleSubmit}>
          <div className="input-group">
            <span className="input-group-text">prompt$</span>
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
          <div className="text-danger small mt-2">
            Failed to send command: {(runCommandMutation.error as Error).message}
          </div>
        ) : null}

        {outputError && terminalOutput ? (
          <div className="text-warning small mt-2">
            Output polling degraded: {(outputError as Error).message}
          </div>
        ) : null}
      </div>
    </div>
  );
}
