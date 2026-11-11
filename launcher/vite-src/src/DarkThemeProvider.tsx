import {
  useState,
  useEffect,
  createContext,
  useContext,
  ReactNode,
} from "react";

type Theme = "light" | "dark" | "system";

function getEffectiveTheme(theme: Theme): boolean {
  if (theme === "dark") return true;
  if (theme === "light") return false;
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

interface DarkThemeContextType {
  theme: Theme;
  setTheme: (t: Theme) => void;
  isDark: boolean;
}

const DarkThemeContext = createContext<DarkThemeContextType | null>(null);

export function DarkThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setTheme] = useState<Theme>(
    () => (localStorage.getItem("theme") as Theme) || "system",
  );
  const [isDark, setIsDark] = useState(() => getEffectiveTheme(theme));

  useEffect(() => {
    const apply = (dark: boolean) => {
      document.documentElement.classList.toggle("dark", dark);
      setIsDark(dark);
    };
    apply(getEffectiveTheme(theme));

    if (theme === "system") {
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      const handler = () => apply(getEffectiveTheme("system"));
      mq.addEventListener("change", handler);
      return () => mq.removeEventListener("change", handler);
    }
  }, [theme]);

  const setThemeAndPersist = (t: Theme) => {
    localStorage.setItem("theme", t);
    setTheme(t);
  };

  return (
    <DarkThemeContext.Provider
      value={{ theme, setTheme: setThemeAndPersist, isDark }}
    >
      {children}
    </DarkThemeContext.Provider>
  );
}

export function ThemeToggle() {
  const ctx = useContext(DarkThemeContext);
  if (!ctx)
    throw new Error("ThemeToggle must be used within DarkThemeProvider");
  const { theme, setTheme } = ctx;

  return (
    <div className="flex items-center gap-1 bg-gray-200 dark:bg-gray-800 rounded-lg p-1">
      <button
        onClick={() => setTheme("light")}
        className={`text-lg px-2 py-1 rounded cursor-pointer transition-colors ${theme === "light" ? "bg-white dark:bg-gray-700 shadow-sm text-amber-600" : "text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200"}`}
        title="Light mode"
      >
        ☀️
      </button>
      <button
        onClick={() => setTheme("dark")}
        className={`text-lg px-2 py-1 rounded cursor-pointer transition-colors ${theme === "dark" ? "bg-white dark:bg-gray-700 shadow-sm text-amber-400" : "text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200"}`}
        title="Dark mode"
      >
        🌙
      </button>
      <button
        onClick={() => setTheme("system")}
        className={`text-lg px-2 py-1 rounded cursor-pointer transition-colors ${theme === "system" ? "bg-white dark:bg-gray-700 shadow-sm text-gray-800 dark:text-gray-200" : "text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200"}`}
        title="System theme"
      >
        💻
      </button>
    </div>
  );
}

export function useDarkTheme() {
  const ctx = useContext(DarkThemeContext);
  if (!ctx)
    throw new Error("useDarkTheme must be used within DarkThemeProvider");
  return ctx;
}
