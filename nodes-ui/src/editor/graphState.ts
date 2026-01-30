import { NodeEditor, ClassicPreset } from 'rete';
import { AreaPlugin } from 'rete-area-plugin';
import { Schemes, AreaExtra } from './createEditor';
import { NodeFactory, NodeTypeName } from './nodeFactory';
import { GraphState, SerializedNode, SerializedConnection, TabState, SessionState } from '../types';
import { BaseNode } from '../nodes';
import { v4 as uuidv4 } from 'uuid';

const STORAGE_KEY = 'nodes-ui-session';

// Create a new empty graph state
export function createEmptyGraphState(name: string = 'Untitled'): GraphState {
  const now = new Date().toISOString();
  return {
    id: uuidv4(),
    name,
    nodes: [],
    connections: [],
    createdAt: now,
    updatedAt: now,
  };
}

// Create a new tab state
export function createTabState(name: string = 'New Tab'): TabState {
  return {
    id: uuidv4(),
    name,
    graphState: createEmptyGraphState(name),
    isActive: false,
  };
}

// Create initial session state
export function createInitialSessionState(): SessionState {
  const tab = createTabState('Tab 1');
  tab.isActive = true;
  return {
    tabs: [tab],
    activeTabId: tab.id,
  };
}

// Serialize the current editor state to GraphState
export async function serializeGraph(
  editor: NodeEditor<Schemes>,
  area: AreaPlugin<Schemes, AreaExtra>,
  existingState?: GraphState
): Promise<GraphState> {
  const nodes: SerializedNode[] = [];
  const connections: SerializedConnection[] = [];
  
  // Serialize nodes
  for (const node of editor.getNodes()) {
    const nodeView = area.nodeViews.get(node.id);
    const position = nodeView ? { x: nodeView.position.x, y: nodeView.position.y } : { x: 0, y: 0 };
    
    // Try to use BaseNode serialize if available, otherwise use basic info
    const baseNode = node as any;
    const serialized = typeof baseNode.serialize === 'function' 
      ? baseNode.serialize()
      : { type: node.constructor.name, controls: {} };
    
    nodes.push({
      id: node.id,
      type: serialized.type,
      position,
      controls: serialized.controls,
    });
  }
  
  // Serialize connections
  for (const connection of editor.getConnections()) {
    connections.push({
      id: connection.id,
      source: connection.source,
      sourceOutput: connection.sourceOutput,
      target: connection.target,
      targetInput: connection.targetInput,
    });
  }
  
  const now = new Date().toISOString();
  
  return {
    id: existingState?.id || uuidv4(),
    name: existingState?.name || 'Untitled',
    nodes,
    connections,
    createdAt: existingState?.createdAt || now,
    updatedAt: now,
  };
}

// Deserialize and restore editor state from GraphState
export async function deserializeGraph(
  graphState: GraphState,
  editor: NodeEditor<Schemes>,
  area: AreaPlugin<Schemes, AreaExtra>,
  nodeFactory: NodeFactory
): Promise<void> {
  // Clear existing state
  await nodeFactory.clear();
  
  // Map old IDs to new nodes
  const nodeMap = new Map<string, ClassicPreset.Node>();
  
  // Restore nodes
  for (const serializedNode of graphState.nodes) {
    try {
      const node = await nodeFactory.createNode(
        serializedNode.type as NodeTypeName,
        serializedNode.position
      );
      
      // Restore control values if the method exists
      const baseNode = node as any;
      if (typeof baseNode.deserialize === 'function') {
        baseNode.deserialize({ controls: serializedNode.controls });
      }
      
      nodeMap.set(serializedNode.id, node);
    } catch (error) {
      console.error(`Failed to restore node ${serializedNode.type}:`, error);
    }
  }
  
  // Restore connections
  for (const serializedConnection of graphState.connections) {
    const sourceNode = nodeMap.get(serializedConnection.source);
    const targetNode = nodeMap.get(serializedConnection.target);
    
    if (sourceNode && targetNode) {
      try {
        await nodeFactory.createConnection(
          sourceNode,
          serializedConnection.sourceOutput,
          targetNode,
          serializedConnection.targetInput
        );
      } catch (error) {
        console.error('Failed to restore connection:', error);
      }
    }
  }
}

// Save graph state to JSON file
export function downloadGraphAsJson(graphState: GraphState): void {
  const json = JSON.stringify(graphState, null, 2);
  const blob = new Blob([json], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  
  const a = document.createElement('a');
  a.href = url;
  a.download = `${graphState.name.replace(/\s+/g, '-').toLowerCase()}.json`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

// Load graph state from JSON file
export async function loadGraphFromJson(file: File): Promise<GraphState> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = (event) => {
      try {
        const json = event.target?.result as string;
        const graphState = JSON.parse(json) as GraphState;
        resolve(graphState);
      } catch (error) {
        reject(new Error('Failed to parse graph JSON'));
      }
    };
    reader.onerror = () => reject(new Error('Failed to read file'));
    reader.readAsText(file);
  });
}

// Session storage helpers
export function saveSessionToStorage(session: SessionState): void {
  if (typeof window !== 'undefined') {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  }
}

export function loadSessionFromStorage(): SessionState | null {
  if (typeof window === 'undefined') {
    return null;
  }
  
  const stored = localStorage.getItem(STORAGE_KEY);
  if (stored) {
    try {
      return JSON.parse(stored) as SessionState;
    } catch {
      return null;
    }
  }
  return null;
}

export function clearSessionStorage(): void {
  if (typeof window !== 'undefined') {
    localStorage.removeItem(STORAGE_KEY);
  }
}
