interface LogProps {
  stdout: string[];
  stderr: string[];
  label?: string;
}

export function Log({ stdout, stderr, label = "Log" }: LogProps) {
  if (stdout.length === 0 && stderr.length === 0) return null;
  return (
    <details className="text-xs">
      <summary className="cursor-pointer text-gray-500 dark:text-gray-400">
        {label}
      </summary>
      <pre className="mt-1 bg-gray-200 dark:bg-gray-900 rounded-lg p-2 text-xs overflow-auto max-h-32 whitespace-pre-wrap">
        {stdout.join("")}
        {stderr.join("")}
      </pre>
    </details>
  );
}
