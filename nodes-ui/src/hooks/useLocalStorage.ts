import { useState, useEffect, useCallback } from 'react';

const STORAGE_PREFIX = 'nodes-ui__';

export function useLocalStorage<T>(
  key: string,
  initialValue: T
): [T, (value: T | ((prev: T) => T)) => void] {
  const prefixedKey = STORAGE_PREFIX + key;
  
  const [storedValue, setStoredValue] = useState<T>(initialValue);
  const [isInitialized, setIsInitialized] = useState(false);
  
  // Load from localStorage on mount
  useEffect(() => {
    if (typeof window === 'undefined') return;
    
    try {
      const item = localStorage.getItem(prefixedKey);
      if (item) {
        setStoredValue(JSON.parse(item));
      }
    } catch (error) {
      console.error('Failed to load from localStorage:', error);
    }
    setIsInitialized(true);
  }, [prefixedKey]);
  
  // Save to localStorage on change
  const setValue = useCallback(
    (value: T | ((prev: T) => T)) => {
      setStoredValue((prev) => {
        const newValue = value instanceof Function ? value(prev) : value;
        
        if (typeof window !== 'undefined' && isInitialized) {
          try {
            localStorage.setItem(prefixedKey, JSON.stringify(newValue));
          } catch (error) {
            console.error('Failed to save to localStorage:', error);
          }
        }
        
        return newValue;
      });
    },
    [prefixedKey, isInitialized]
  );
  
  return [storedValue, setValue];
}

export function clearLocalStorage(key: string): void {
  if (typeof window !== 'undefined') {
    localStorage.removeItem(STORAGE_PREFIX + key);
  }
}

export function getAllLocalStorageKeys(): string[] {
  if (typeof window === 'undefined') return [];
  
  const keys: string[] = [];
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key?.startsWith(STORAGE_PREFIX)) {
      keys.push(key.slice(STORAGE_PREFIX.length));
    }
  }
  return keys;
}
