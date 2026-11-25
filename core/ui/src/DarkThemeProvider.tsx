import {
  createContext,
  createEffect,
  createMemo,
  createSignal,
  For,
  onSettled,
  useContext,
  type Accessor,
  type ParentComponent,
} from "solid-js";

type Theme = "light" | "dark" | "system";

interface DarkThemeContextType {
  theme: Accessor<Theme>;
  setTheme: (t: Theme) => void;
  isDark: Accessor<boolean>;
}

const DarkThemeContext = createContext<DarkThemeContextType>();

export const DarkThemeProvider: ParentComponent = (props) => {
  const [theme, setTheme] = createSignal<Theme>(
    (localStorage.getItem("theme") as Theme) || "system",
  );

  // The OS preference is its own signal; "system" just reads it. No effect has
  // to write isDark back, and the listener lives for the provider's lifetime.
  const mq = window.matchMedia("(prefers-color-scheme: dark)");
  const [systemDark, setSystemDark] = createSignal(mq.matches);
  onSettled(() => {
    const handler = () => setSystemDark(mq.matches);
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  });

  const isDark = createMemo(
    () => theme() === "dark" || (theme() === "system" && systemDark()),
  );

  createEffect(isDark, (dark) => {
    document.documentElement.classList.toggle("dark", dark);
  });

  const setThemeAndPersist = (t: Theme) => {
    localStorage.setItem("theme", t);
    setTheme(t);
  };

  return (
    <DarkThemeContext
      value={{ theme, setTheme: setThemeAndPersist, isDark }}
    >
      {props.children}
    </DarkThemeContext>
  );
};

const OPTIONS: { theme: Theme; icon: string; title: string; active: string }[] = [
  {
    theme: "light",
    icon: "☀️",
    title: "Light mode",
    active: "bg-white dark:bg-gray-700 shadow-sm text-amber-600",
  },
  {
    theme: "dark",
    icon: "🌙",
    title: "Dark mode",
    active: "bg-white dark:bg-gray-700 shadow-sm text-amber-400",
  },
  {
    theme: "system",
    icon: "💻",
    title: "System theme",
    active: "bg-white dark:bg-gray-700 shadow-sm text-gray-800 dark:text-gray-200",
  },
];

export function ThemeToggle() {
  // useContext is typed T: with no provider above it throws on its own.
  const ctx = useContext(DarkThemeContext);

  return (
    <div class="flex items-center gap-1 bg-gray-200 dark:bg-gray-800 rounded-lg p-1">
      <For each={OPTIONS}>
        {(o) => (
          <button
            onClick={() => ctx.setTheme(o.theme)}
            class={[
              "text-lg px-2 py-1 rounded cursor-pointer transition-colors",
              ctx.theme() === o.theme
                ? o.active
                : "text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200",
            ]}
            title={o.title}
          >
            {o.icon}
          </button>
        )}
      </For>
    </div>
  );
}

export function useDarkTheme() {
  return useContext(DarkThemeContext);
}
