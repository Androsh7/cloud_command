import { useMutation } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { useParams } from "react-router-dom";
import { executeCommand } from "../components/Api";

type ShellEntry = {
  command: string;
  stdout: string;
  stderr: string;
  exitCode: number | null;
  error?: string;
};

export default function InteractiveShell() {
  const { agentName } = useParams<{ agentName: string }>();
  const [command, setCommand] = useState("");
  const [history, setHistory] = useState<ShellEntry[]>([]);

  const runCommandMutation = useMutation({
    mutationFn: (cmd: string) => {
      if (!agentName) {
        throw new Error("Agent name is missing in route.");
      }
      return executeCommand(agentName, cmd);
    },
    onSuccess: (result, cmd) => {
      setHistory((previous) => [
        ...previous,
        {
          command: cmd,
          stdout: result.stdout,
          stderr: result.stderr,
          exitCode: result.exit_code,
        },
      ]);
    },
    onError: (error, cmd) => {
      setHistory((previous) => [
        ...previous,
        {
          command: cmd,
          stdout: "",
          stderr: "",
          exitCode: null,
          error: (error as Error).message,
        },
      ]);
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
          className="card-body bg-dark text-light"
          style={{ minHeight: "360px", fontFamily: "monospace" }}
        >
          {history.length === 0 ? (
            <p className="text-secondary mb-0">
              Enter a command to run it on {agentName}.
            </p>
          ) : (
            history.map((entry, index) => (
              <div key={`${entry.command}-${index}`} className="mb-3">
                <div>
                  <span className="text-info">{agentName}</span>:~$ {entry.command}
                </div>
                {entry.stdout ? (
                  <pre className="mb-1 text-light">{entry.stdout}</pre>
                ) : null}
                {entry.stderr ? (
                  <pre className="mb-1 text-warning">{entry.stderr}</pre>
                ) : null}
                {entry.error ? (
                  <div className="text-danger">Request failed: {entry.error}</div>
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
    </div>
  );
}
