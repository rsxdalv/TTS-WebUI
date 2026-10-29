interface StatusProps {
  label: string;
  exitCode: number | null;
  running?: boolean;
}

export function Status({ label, exitCode, running = false }: StatusProps) {
  if (running) {
    return (
      <p className="text-xs text-center text-gray-500 dark:text-gray-400">
        {label} ⟳ running...
      </p>
    );
  }
  if (exitCode === null) return null;
  return (
    <p className="text-xs text-center text-gray-500 dark:text-gray-400">
      {label} {exitCode === 0 ? "✓ completed" : `✗ failed (exit ${exitCode})`}
    </p>
  );
}
