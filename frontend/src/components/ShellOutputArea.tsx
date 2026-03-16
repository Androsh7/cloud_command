import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useRef,
} from "react";

export interface ShellOutputHandle {
  scrollToBottom: () => void;
}

interface ShellOutputAreaProps {
  output: string | null | undefined;
  isLoading: boolean;
  error: Error | null;
}

const ShellOutputArea = forwardRef<ShellOutputHandle, ShellOutputAreaProps>(
  ({ output, isLoading, error }, ref) => {
    const containerRef = useRef<HTMLDivElement | null>(null);
    const bottomAnchorRef = useRef<HTMLDivElement | null>(null);

    const scrollToBottom = useCallback(() => {
      const container = containerRef.current;
      if (!container) return;
      container.scrollTop = container.scrollHeight;
      bottomAnchorRef.current?.scrollIntoView({ block: "end" });
    }, []);

    useImperativeHandle(ref, () => ({ scrollToBottom }), [scrollToBottom]);

    useLayoutEffect(() => {
      scrollToBottom();
    }, [output, scrollToBottom]);

    useEffect(() => {
      const container = containerRef.current;
      if (!container) return;
      const observer = new MutationObserver(() => scrollToBottom());
      observer.observe(container, {
        childList: true,
        subtree: true,
        characterData: true,
      });
      return () => observer.disconnect();
    }, [scrollToBottom]);

    return (
      <div ref={containerRef} className="shell-output">
        {isLoading && !output ? (
          <p className="text-secondary mb-0">Loading terminal output...</p>
        ) : error && !output ? (
          <p className="text-danger mb-0">
            Failed to fetch output: {error.message}
          </p>
        ) : output ? (
          <pre className="shell-output-pre">{output}</pre>
        ) : (
          <p className="text-secondary mb-0">
            Enter a command to start interacting with this shell session.
          </p>
        )}
        <div ref={bottomAnchorRef} />
      </div>
    );
  },
);

ShellOutputArea.displayName = "ShellOutputArea";

export default ShellOutputArea;
