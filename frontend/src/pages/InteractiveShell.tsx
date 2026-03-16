import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";
import {
  getShellOutput,
  getShellStatistics,
  listShellSessions,
  sendCtrlCShellCommand,
  sendShellCommand,
} from "../components/Api";
import FileTransferDialog from "../components/FileTransferDialog";
import ShellOutputArea, {
  type ShellOutputHandle,
} from "../components/ShellOutputArea";
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
  const [transferDialog, setTransferDialog] = useState<
    "upload" | "download" | null
  >(null);
  const [fileTransferMessage, setFileTransferMessage] = useState<string | null>(
    null,
  );

  const isFirstFetch = useRef(true);
  const lastNonNullOutput = useRef<string | null>(null);
  const shellOutputRef = useRef<ShellOutputHandle | null>(null);

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
    data: displayedOutput,
    isLoading: isOutputLoading,
    error: outputError,
    refetch: refetchOutput,
  } = useQuery({
    queryKey: ["shellOutput", uuid],
    queryFn: async () => {
      if (!uuid) throw new Error("Shell session UUID is missing in route.");
      const showExisting = isFirstFetch.current;
      isFirstFetch.current = false;
      const result = await getShellOutput(uuid, showExisting);
      if (result !== null) {
        lastNonNullOutput.current = result;
      }
      return lastNonNullOutput.current;
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
      if (!uuid) throw new Error("Shell session UUID is missing in route.");
      return getShellStatistics(uuid);
    },
    enabled: Boolean(uuid) && isWindowFocused,
    refetchInterval: 5000,
  });

  const { data: shellSessions } = useQuery({
    queryKey: ["shellSessions", uuid],
    queryFn: () => listShellSessions(),
    enabled: Boolean(uuid) && isWindowFocused,
    refetchInterval: 5000,
  });

  const activeAgentName = shellSessions?.find(
    (s) => s.uuid === uuid,
  )?.agent_name;

  const transferStatusMessage = !activeAgentName
    ? "This shell session was not found. File transfer is unavailable."
    : fileTransferMessage;
  const transferStatusClass = !activeAgentName ? "text-warning" : "text-info";

  const runCommandMutation = useMutation({
    mutationFn: (cmd: string) => {
      if (!uuid) throw new Error("Shell session UUID is missing in route.");
      return sendShellCommand(uuid, cmd);
    },
    onSuccess: () => {
      void refetchOutput();
      window.setTimeout(() => void refetchOutput(), 120);
      window.setTimeout(() => void refetchOutput(), 300);
      shellOutputRef.current?.scrollToBottom();
    },
  });

  const sendCtrlCMutation = useMutation({
    mutationFn: () => {
      if (!uuid) throw new Error("Shell session UUID is missing in route.");
      return sendCtrlCShellCommand(uuid);
    },
    onSuccess: () => {
      void refetchOutput();
      window.setTimeout(() => void refetchOutput(), 120);
      window.setTimeout(() => void refetchOutput(), 300);
      shellOutputRef.current?.scrollToBottom();
    },
  });

  useEffect(() => {
    if (!runCommandMutation.isPending) {
      setRunningFrame(0);
      return;
    }
    const intervalId = window.setInterval(() => {
      setRunningFrame((prev) => (prev + 1) % 3);
    }, 450);
    return () => window.clearInterval(intervalId);
  }, [runCommandMutation.isPending]);

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cmd = command.trim();
    if (!cmd || runCommandMutation.isPending) return;
    setCommand("");
    shellOutputRef.current?.scrollToBottom();
    runCommandMutation.mutate(cmd);
  }

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
      <ShellOutputArea
        ref={shellOutputRef}
        output={displayedOutput}
        isLoading={isOutputLoading}
        error={outputError ?? null}
      />

      <div className="shell-bottom-strip">
        <div className="shell-session-label">
          Session: <code>{uuid}</code>
        </div>

        <div className="shell-stats-row">
          {isStatisticsLoading ? (
            <span className="badge bg-warning text-dark">Loading stats...</span>
          ) : statisticsError ? (
            <span className="badge bg-danger text-dark">
              Stats unavailable: {(statisticsError as Error).message}
            </span>
          ) : agentStatistics ? (
            <>
              <span className="badge bg-info text-dark">
                Uptime:{" "}
                {dayjs
                  .duration(agentStatistics.uptime_seconds, "seconds")
                  .humanize()}
              </span>
              <span className="badge bg-info text-dark">
                CPU: {agentStatistics.cpu_usage}
              </span>
              <span className="badge bg-info text-dark">
                RAM: {agentStatistics.ram_usage}
              </span>
              <span className="badge bg-info text-dark">
                Disk: {agentStatistics.disk_usage}
              </span>
            </>
          ) : (
            <span className="badge bg-info text-dark">No stats available</span>
          )}

          {runCommandMutation.isPending ? (
            <span className="badge bg-warning text-dark">
              Running{".".repeat(runningFrame + 1)}
            </span>
          ) : null}
        </div>

        <div className="shell-transfer-row">
          <button
            type="button"
            className="btn btn-outline-info btn-sm"
            onClick={() => activeAgentName && setTransferDialog("upload")}
            disabled={!activeAgentName}
          >
            Upload File
          </button>
          <button
            type="button"
            className="btn btn-outline-success btn-sm"
            onClick={() => activeAgentName && setTransferDialog("download")}
            disabled={!activeAgentName}
          >
            Download File
          </button>
          <button
            type="button"
            className="btn btn-outline-danger btn-sm"
            onClick={() => sendCtrlCMutation.mutate()}
            disabled={sendCtrlCMutation.isPending}
          >
            {sendCtrlCMutation.isPending ? "Sending Ctrl+C..." : "Send Ctrl+C"}
          </button>

          {transferStatusMessage ? (
            <span
              className={`small shell-transfer-message ${transferStatusClass}`}
            >
              {transferStatusMessage}
            </span>
          ) : null}
        </div>

        <form onSubmit={handleSubmit}>
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
          <div className="text-danger small mt-2">
            Failed to send command:{" "}
            {(runCommandMutation.error as Error).message}
          </div>
        ) : null}

        {sendCtrlCMutation.error ? (
          <div className="text-danger small mt-2">
            Failed to send Ctrl+C:{" "}
            {(sendCtrlCMutation.error as Error).message}
          </div>
        ) : null}

        {outputError && displayedOutput ? (
          <div className="text-warning small mt-2">
            Output polling degraded: {(outputError as Error).message}
          </div>
        ) : null}
      </div>

      {transferDialog && activeAgentName ? (
        <FileTransferDialog
          agentName={activeAgentName}
          mode={transferDialog}
          onClose={() => setTransferDialog(null)}
          onMessage={setFileTransferMessage}
        />
      ) : null}
    </div>
  );
}
