interface ButtonProps {
  label: string;
  isRunning: boolean;
  onClick: () => void;
  disabled?: boolean;
  variant?: "green" | "red" | "purple" | "blue" | "gray";
}

export function Button({
  label,
  onClick,
  disabled = false,
  variant = "green",
}: ButtonProps) {
  const colors: Record<string, string> = {
    green: "bg-green-600 hover:bg-green-500",
    red: "bg-red-600 hover:bg-red-500",
    purple: "bg-purple-600 hover:bg-purple-500",
    blue: "bg-blue-600 hover:bg-blue-500",
    gray: "bg-gray-500 hover:bg-gray-600",
  };

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={`w-full py-2 px-3 rounded-lg font-medium transition-colors cursor-pointer text-white disabled:bg-gray-400 disabled:cursor-not-allowed ${colors[variant]}`}
    >
      {label}
    </button>
  );
}
