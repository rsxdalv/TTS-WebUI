import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

const KEY = "tts-webui-theme";

export const readTheme = (): Theme => {
  try {
    return localStorage.getItem(KEY) === "dark" ? "dark" : "light";
  } catch {
    return "light";
  }
};

/** Puts the theme on <html>: Tailwind's `dark:` classes and the .dark variables in globals.css follow it. */
export const applyTheme = (theme: Theme) => {
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
};

export const useTheme = () => {
  const [theme, setTheme] = useState<Theme>(readTheme);

  useEffect(() => {
    applyTheme(theme);
  }, [theme]);

  // Another tab changed it.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => e.key === KEY && setTheme(readTheme());
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, []);

  const set = useCallback((next: Theme) => {
    try {
      localStorage.setItem(KEY, next);
    } catch {
      // Private mode: the choice lasts until the page closes.
    }
    setTheme(next);
  }, []);

  return { theme, setTheme: set, toggle: () => set(theme === "dark" ? "light" : "dark") };
};
