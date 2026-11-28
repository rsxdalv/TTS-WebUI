import React from "react";
import { MoonIcon, SunIcon } from "lucide-react";
import { useTheme } from "@/lib/theme";

/** Switches between the light and dark theme; the choice is remembered. */
export const ThemeToggle = () => {
  const { theme, toggle } = useTheme();
  const label = theme === "dark" ? "Switch to light mode" : "Switch to dark mode";
  return (
    <button
      type="button"
      onClick={toggle}
      aria-label={label}
      title={label}
      className="inline-flex h-8 w-8 items-center justify-center rounded border text-gray-600 hover:bg-gray-100 dark:hover:bg-gray-800"
    >
      {theme === "dark" ? <SunIcon className="h-4 w-4" /> : <MoonIcon className="h-4 w-4" />}
    </button>
  );
};
