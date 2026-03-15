import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { downloadAgentFile, uploadAgentFile } from "./Api";

interface FileTransferDialogProps {
  agentName: string;
  mode: "upload" | "download";
  onClose: () => void;
  onMessage: (msg: string) => void;
}

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

export default function FileTransferDialog({
  agentName,
  mode,
  onClose,
  onMessage,
}: FileTransferDialogProps) {
  const uploadFileInputRef = useRef<HTMLInputElement | null>(null);
  const [selectedUploadFile, setSelectedUploadFile] = useState<File | null>(null);
  const [uploadPath, setUploadPath] = useState("/tmp/");
  const [downloadPath, setDownloadPath] = useState("");

  const uploadFileMutation = useMutation({
    mutationFn: ({
      file,
      destinationPath,
    }: {
      file: File;
      destinationPath: string;
    }) => uploadAgentFile(agentName, destinationPath, file),
    onSuccess: (result, variables) => {
      onMessage(`Uploaded ${variables.file.name} to ${result.destination_path}`);
      setSelectedUploadFile(null);
      if (uploadFileInputRef.current) {
        uploadFileInputRef.current.value = "";
      }
      onClose();
    },
    onError: (error) => {
      onMessage(`Upload failed: ${(error as Error).message}`);
    },
  });

  const downloadFileMutation = useMutation({
    mutationFn: (path: string) => downloadAgentFile(agentName, path),
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
      onMessage(`Downloaded ${trimmedPath}`);
      onClose();
    },
    onError: (error) => {
      onMessage(`Download failed: ${(error as Error).message}`);
    },
  });

  const isPending = uploadFileMutation.isPending || downloadFileMutation.isPending;

  function handleClose() {
    if (!isPending) {
      onClose();
    }
  }

  function handleUploadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedUploadFile || isPending) return;
    const destinationPath = resolveUploadDestinationPath(
      uploadPath,
      selectedUploadFile.name,
    );
    uploadFileMutation.mutate({ file: selectedUploadFile, destinationPath });
  }

  function handleDownloadSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedPath = downloadPath.trim();
    if (!trimmedPath || isPending) return;
    downloadFileMutation.mutate(trimmedPath);
  }

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        handleClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isPending]);

  return (
    <div
      className="shell-transfer-dialog-backdrop"
      role="presentation"
      onClick={handleClose}
    >
      <div
        className="shell-transfer-dialog card"
        role="dialog"
        aria-modal="true"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="card-body">
          {mode === "upload" ? (
            <>
              <h5 className="card-title mb-3">Upload File</h5>
              <form onSubmit={handleUploadSubmit}>
                <div className="mb-2">
                  <label
                    className="form-label small mb-1"
                    htmlFor="upload-file"
                  >
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
                  <label
                    className="form-label small mb-1"
                    htmlFor="upload-path"
                  >
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
                    onClick={handleClose}
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
                  <label
                    className="form-label small mb-1"
                    htmlFor="download-path"
                  >
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
                    onClick={handleClose}
                    disabled={downloadFileMutation.isPending}
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="btn btn-success btn-sm"
                    disabled={
                      !downloadPath.trim() || downloadFileMutation.isPending
                    }
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
  );
}
