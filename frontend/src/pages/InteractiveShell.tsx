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
  downloadAgentFile,
  getShellOutput,
  getShellStatistics,
  listShellSessions,
  sendShellCommand,
  uploadAgentFile,
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
  const uploadFileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedUploadFile, setSelectedUploadFile] = useState<File | null>(null);
  const [uploadPath, setUploadPath] = useState("/tmp/");
  const [downloadPath, setDownloadPath] = useState("");
  const [transferDialog, setTransferDialog] = useState<
    "upload" | "download" | null
  >(null);
  const [fileTransferMessage, setFileTransferMessage] = useState<string | null>(
    null,
  );

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

  const { data: shellSessions } = useQuery({
    queryKey: ["shellSessions", uuid],
    queryFn: () => listShellSessions(),
    enabled: Boolean(uuid) && isWindowFocused,
    refetchInterval: 5000,
  });

  const activeAgentName = shellSessions?.find(
    (shellSession) => shellSession.uuid === uuid,
  )?.agent_name;
  const transferStatusMessage = !activeAgentName
    ? "This shell session was not found. File transfer is unavailable."
    : fileTransferMessage;
  const transferStatusClass = !activeAgentName ? "text-warning" : "text-info";

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

  const uploadFileMutation = useMutation({
    mutationFn: ({
      file,
      destinationPath,
    }: {
      file: File;
      destinationPath: string;
    }) => {
      if (!activeAgentName) {
        throw new Error("Unable to resolve agent for this shell session.");
      }
      return uploadAgentFile(activeAgentName, destinationPath, file);
    },
    onSuccess: (result, variables) => {
      setFileTransferMessage(
        `Uploaded ${variables.file.name} to ${result.destination_path}`,
      );
      setSelectedUploadFile(null);
      setTransferDialog(null);
      if (uploadFileInputRef.current) {
        uploadFileInputRef.current.value = "";
      }
    },
    onError: (error) => {
      setFileTransferMessage(`Upload failed: ${(error as Error).message}`);
    },
  });

  const downloadFileMutation = useMutation({
    mutationFn: (path: string) => {
      if (!activeAgentName) {
        throw new Error("Unable to resolve agent for this shell session.");
      }
      return downloadAgentFile(activeAgentName, path);
    },
    onSuccess: (blob, path) => {
      const trimmedPath = path.trim();
      const filename = trimmedPath.split(/[\\/]/).pop() || "download.bin";
      const blobUrl = window.URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = blobUrl;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.URL.revokeObjectURL(blobUrl);
      setFileTransferMessage(`Downloaded ${trimmedPath}`);
      setTransferDialog(null);
    },
    onError: (error) => {
      setFileTransferMessage(`Download failed: ${(error as Error).message}`);
    },
  });

  function resolveUploadDestinationPath(path: string, filename: string): string {
    const trimmedPath = path.trim();
    if (!trimmedPath) {
      return `/tmp/${filename}`;
    }
    if (trimmedPath.endsWith("/") || trimmedPath.endsWith("\\")) {
      return `${trimmedPath}${filename}`;
    }
    return trimmedPath;
  }

  function openUploadDialog() {
    if (!activeAgentName || uploadFileMutation.isPending || downloadFileMutation.isPending) {
      return;
    }
    setTransferDialog("upload");
  }

  function openDownloadDialog() {
    if (!activeAgentName || uploadFileMutation.isPending || downloadFileMutation.isPending) {
      return;
    }
    setTransferDialog("download");
  }

  function handleUploadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (
      !selectedUploadFile ||
      uploadFileMutation.isPending ||
      downloadFileMutation.isPending
    ) {
      return;
    }

    setFileTransferMessage(null);
    const destinationPath = resolveUploadDestinationPath(
      uploadPath,
      selectedUploadFile.name,
    );
    uploadFileMutation.mutate({ file: selectedUploadFile, destinationPath });
  }

  function handleDownloadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedPath = downloadPath.trim();
    if (
      !trimmedPath ||
      downloadFileMutation.isPending ||
      uploadFileMutation.isPending
    ) {
      return;
    }

    setFileTransferMessage(null);
    downloadFileMutation.mutate(trimmedPath);
  }

  function closeTransferDialog() {
    if (uploadFileMutation.isPending || downloadFileMutation.isPending) {
      return;
    }
    setTransferDialog(null);
  }

  useEffect(() => {
    if (!transferDialog) {
      return;
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closeTransferDialog();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [transferDialog, uploadFileMutation.isPending, downloadFileMutation.isPending]);

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
      <div ref={outputContainerRef} className="shell-output">
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

      <div className="shell-bottom-strip">
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

        <div className="shell-transfer-row">
          <button
            type="button"
            className="btn btn-outline-info btn-sm"
            onClick={openUploadDialog}
            disabled={
              !activeAgentName ||
              uploadFileMutation.isPending ||
              downloadFileMutation.isPending
            }
          >
            {uploadFileMutation.isPending ? "Uploading..." : "Upload File"}
          </button>
          <button
            type="button"
            className="btn btn-outline-success btn-sm"
            onClick={openDownloadDialog}
            disabled={
              !activeAgentName ||
              uploadFileMutation.isPending ||
              downloadFileMutation.isPending
            }
          >
            {downloadFileMutation.isPending ? "Downloading..." : "Download File"}
          </button>

          {transferStatusMessage ? (
            <span className={`small shell-transfer-message ${transferStatusClass}`}>
              {transferStatusMessage}
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

      {transferDialog ? (
        <div
          className="shell-transfer-dialog-backdrop"
          role="presentation"
          onClick={closeTransferDialog}
        >
          <div
            className="shell-transfer-dialog card"
            role="dialog"
            aria-modal="true"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="card-body">
              {transferDialog === "upload" ? (
                <>
                  <h5 className="card-title mb-3">Upload File</h5>
                  <form onSubmit={handleUploadSubmit}>
                    <div className="mb-2">
                      <label className="form-label small mb-1" htmlFor="upload-file">
                        File
                      </label>
                      <input
                        ref={uploadFileInputRef}
                        id="upload-file"
                        type="file"
                        className="form-control form-control-sm"
                        onChange={(event) => {
                          setSelectedUploadFile(event.target.files?.[0] ?? null);
                        }}
                        disabled={uploadFileMutation.isPending}
                      />
                    </div>
                    <div className="mb-3">
                      <label className="form-label small mb-1" htmlFor="upload-path">
                        Destination Path
                      </label>
                      <input
                        id="upload-path"
                        className="form-control form-control-sm"
                        value={uploadPath}
                        onChange={(event) => setUploadPath(event.target.value)}
                        placeholder="/tmp/ or /tmp/file.txt"
                        disabled={uploadFileMutation.isPending}
                      />
                    </div>
                    <div className="d-flex justify-content-end gap-2">
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        onClick={closeTransferDialog}
                        disabled={uploadFileMutation.isPending}
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        className="btn btn-info btn-sm"
                        disabled={!selectedUploadFile || uploadFileMutation.isPending}
                      >
                        {uploadFileMutation.isPending ? "Uploading..." : "Upload"}
                      </button>
                    </div>
                  </form>
                </>
              ) : (
                <>
                  <h5 className="card-title mb-3">Download File</h5>
                  <form onSubmit={handleDownloadSubmit}>
                    <div className="mb-3">
                      <label className="form-label small mb-1" htmlFor="download-path">
                        Remote Path
                      </label>
                      <input
                        id="download-path"
                        className="form-control form-control-sm"
                        value={downloadPath}
                        onChange={(event) => setDownloadPath(event.target.value)}
                        placeholder="/tmp/file.txt"
                        disabled={downloadFileMutation.isPending}
                      />
                    </div>
                    <div className="d-flex justify-content-end gap-2">
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        onClick={closeTransferDialog}
                        disabled={downloadFileMutation.isPending}
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        className="btn btn-success btn-sm"
                        disabled={!downloadPath.trim() || downloadFileMutation.isPending}
                      >
                        {downloadFileMutation.isPending
                          ? "Downloading..."
                          : "Download"}
                      </button>
                    </div>
                  </form>
                </>
              )}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
