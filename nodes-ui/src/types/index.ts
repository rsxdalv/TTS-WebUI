import { ClassicPreset } from 'rete';

// Socket types for different data types
export class AudioSocket extends ClassicPreset.Socket {
  constructor() {
    super('Audio');
  }
}

export class TextSocket extends ClassicPreset.Socket {
  constructor() {
    super('Text');
  }
}

export class NumberSocket extends ClassicPreset.Socket {
  constructor() {
    super('Number');
  }
}

export class AnySocket extends ClassicPreset.Socket {
  constructor() {
    super('Any');
  }
}

// Singleton instances for socket types
export const audioSocket = new AudioSocket();
export const textSocket = new TextSocket();
export const numberSocket = new NumberSocket();
export const anySocket = new AnySocket();

// Audio data type
export interface AudioData {
  url: string;
  name?: string;
  duration?: number;
  sampleRate?: number;
  blob?: Blob;
}

// Node execution context
export interface NodeExecutionContext {
  getInputData: <T>(key: string) => T | undefined;
  setOutputData: <T>(key: string, data: T) => void;
}

// Custom node configuration from JS file
export interface CustomNodeConfig {
  id: string;
  name: string;
  category: string;
  inputs: Array<{
    key: string;
    label: string;
    socket: 'audio' | 'text' | 'number' | 'any';
  }>;
  outputs: Array<{
    key: string;
    label: string;
    socket: 'audio' | 'text' | 'number' | 'any';
  }>;
  controls?: Array<{
    key: string;
    label: string;
    type: 'text' | 'number' | 'select' | 'slider';
    options?: string[];
    min?: number;
    max?: number;
    step?: number;
    default?: any;
  }>;
  execute: (inputs: Record<string, any>, controls: Record<string, any>) => Promise<Record<string, any>>;
}

// Graph state for save/load
export interface GraphState {
  id: string;
  name: string;
  nodes: SerializedNode[];
  connections: SerializedConnection[];
  createdAt: string;
  updatedAt: string;
}

export interface SerializedNode {
  id: string;
  type: string;
  position: { x: number; y: number };
  controls: Record<string, any>;
}

export interface SerializedConnection {
  id: string;
  source: string;
  sourceOutput: string;
  target: string;
  targetInput: string;
}

// Tab state
export interface TabState {
  id: string;
  name: string;
  graphState: GraphState;
  isActive: boolean;
}

// Session state
export interface SessionState {
  tabs: TabState[];
  activeTabId: string;
}
