"use client";
import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { Command, CommandDialog } from "@/components/ui/command";
import {
  CommandInput,
  CommandList,
  CommandEmpty,
  CommandGroup,
  CommandItem,
} from "@/components/ui/command";
import { Route, routes } from "./Header";

function CommandShortcut() {
  return (
    <p className="text-sm text-muted-foreground">
      Press{" "}
      <kbd className="pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground opacity-100">
        <span className="text-xs">⌘</span>K
      </kbd>
    </p>
  );
}

export function RouteCommands({}) {
  const [isFocused, setIsFocused] = useState(false);
  const router = useRouter();
  const allRoutes: (Route & { parent?: Route })[] = routes.reduce(
    (acc, route) => [
      ...acc,
      route,
      ...(route.subroutes || []).reduce(
        (acc, subroute) => [
          ...acc,
          { ...subroute, parent: route },
          ...(subroute.subroutes || []).map((subsubroute) => ({
            ...subsubroute,
            parent: subroute,
          })),
        ],
        [] as Route[]
      ),
    ],
    [] as Route[]
  );

  return (
    <Command
      onFocus={() => setIsFocused(true)}
      onBlur={() => setIsFocused(false)}
      className="[&_[cmdk-group-heading]]:px-2 [&_[cmdk-group-heading]]:font-medium [&_[cmdk-group-heading]]:text-muted-foreground [&_[cmdk-group]:not([hidden])_~[cmdk-group]]:pt-0 [&_[cmdk-group]]:px-2 [&_[cmdk-input-wrapper]_svg]:h-5 [&_[cmdk-input-wrapper]_svg]:w-5 [&_[cmdk-input]]:h-12 [&_[cmdk-item]]:px-2 [&_[cmdk-item]]:py-3 [&_[cmdk-item]_svg]:h-5 [&_[cmdk-item]_svg]:w-5"
    >
      <CommandInput placeholder="Type a command or search..." />
      <CommandList className={isFocused ? "" : "hidden"}>
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigation">
          {allRoutes.map((route) => (
            <CommandItem
              key={route.href}
              onSelect={() => router.push(route.href)}
            >
              {route.parent ? route.parent.text + " > " : ""}
              {route.text}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </Command>
  );
}

function CommandDialogContent({
  setOpen,
}: {
  setOpen: (open: boolean) => void;
}) {
  const [isFocused, setIsFocused] = useState(false);
  const router = useRouter();

  const runCommand = (command: () => void) => {
    setOpen(false);
    command();
  };

  return (
    <>
      <CommandInput
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
        placeholder="Type a command or search..."
      />
      <CommandList
        onFocus={() => setIsFocused(true)}
        // onBlur={() => setIsFocused(false)}
        className={isFocused ? "" : "hidden"}
      >
        <CommandEmpty>No results found.</CommandEmpty>
        <CommandGroup heading="Navigation">
          {routes.map((route) => (
            <CommandItem
              key={route.href + route.text}
              onSelect={() => runCommand(() => router.push(route.href))}
            >
              {route.text}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </>
  );
}

export function CommandBoxComponent() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const down = (e: KeyboardEvent) => {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        setOpen((open) => !open);
      }
    };

    document.addEventListener("keydown", down);
    return () => document.removeEventListener("keydown", down);
  }, []);

  return (
    <>
      <CommandShortcut />
      <CommandDialog open={open} onOpenChange={setOpen}>
        <CommandDialogContent setOpen={setOpen} />
      </CommandDialog>
    </>
  );
}
