import { useMutation } from "@tanstack/react-query";
import { useEffect, useState, type FormEvent } from "react";
import { executeCommand } from "./Api";
import type { CommandResultModel } from "./Models";

interface RunCommandDialogProps {
  agentName: string;
  onClose: () => void;
  onStatusMessage: (msg: string) => void;
}

export default function RunCommandDialog({
  agentName,
  onClose,
  onStatusMessage,
}: RunCommandDialogProps) {
  const [runCommand, setRunCommand] = useState("");
  const [commandResult, setCommandResult] = useState<CommandResultModel | null>(null);
  const [fullscreen, setFullscreen] = useState(false);

  const runCommandMutation = useMutation({
    mutationFn: (command: string) => executeCommand(agentName, command),
    onSuccess: (result) => {
      setCommandResult(result);
    },
    onError: (error) => {
      onStatusMessage(`Command failed: ${(error as Error).message}`);
    },
  });

  function handleRunCommand(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCommandResult(null);
    runCommandMutation.mutate(runCommand);
  }

  function handleClose() {
    onClose();
    setFullscreen(false);
  }

  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !runCommandMutation.isPending) {
        handleClose();
      }
    };
    window.addEventListener("keydown", handleEscape);
    return () => window.removeEventListener("keydown", handleEscape);
  }, [runCommandMutation.isPending]);

  return (
    <div
      className="shell-transfer-dialog-backdrop"
      role="presentation"
      onClick={() => {
        if (!runCommandMutation.isPending) {
          handleClose();
        }
      }}
    >
      <div
        className={`${fullscreen ? "run-command-dialog-fullscreen" : "shell-transfer-dialog"} card`}
        role="dialog"
        aria-modal="true"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="card-body">
          <div className="d-flex justify-content-between align-items-center mb-3">
            <h5 className="card-title mb-0">
              Run Command &mdash; {agentName}
            </h5>
            <button
              type="button"
              className="btn btn-outline-secondary btn-sm"
              title={fullscreen ? "Exit fullscreen" : "Fullscreen"}
              onClick={() => setFullscreen((v) => !v)}
            >
              {fullscreen ? "⊡" : "⛶"}
            </button>
          </div>
          <form onSubmit={handleRunCommand}>
            <div className="mb-2">
              <label
                className="form-label small mb-1"
                htmlFor="run-command-input"
              >
                Command
              </label>
              <input
                id="run-command-input"
                className="form-control form-control-sm"
                value={runCommand}
                onChange={(event) => setRunCommand(event.target.value)}
                placeholder="ls -la"
                required
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
              />
            </div>
            <div className="d-flex justify-content-end gap-2 mb-3">
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={handleClose}
                disabled={runCommandMutation.isPending}
              >
                Close
              </button>
              <button
                type="submit"
                className="btn btn-primary btn-sm"
                disabled={runCommandMutation.isPending}
              >
                {runCommandMutation.isPending ? "Running..." : "Run"}
              </button>
            </div>
          </form>
          {commandResult ? (
            <div>
              <div className="d-flex align-items-center gap-2 mb-2">
                <span className="small fw-semibold">Exit code:</span>
                <span
                  className={`badge ${commandResult.exit_code === 0 ? "text-bg-success" : "text-bg-danger"}`}
                >
                  {commandResult.exit_code}
                </span>
              </div>
              {commandResult.stdout ? (
                <div className="mb-2">
                  <div className="small fw-semibold mb-1">stdout</div>
                  <pre
                    className={`run-command-output${fullscreen ? " run-command-output-expanded" : ""}`}
                  >
                    {commandResult.stdout}
                  </pre>
                </div>
              ) : null}
              {commandResult.stderr ? (
                <div className="mb-2">
                  <div className="small fw-semibold mb-1 text-danger">
                    stderr
                  </div>
                  <pre
                    className={`run-command-output text-danger${fullscreen ? " run-command-output-expanded" : ""}`}
                  >
                    {commandResult.stderr}
                  </pre>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
