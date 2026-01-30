import {
  createEmptyGraphState,
  createTabState,
  createInitialSessionState,
} from '../editor/graphState';
import { GraphState, TabState, SessionState } from '../types';

describe('Graph State Management', () => {
  describe('createEmptyGraphState', () => {
    it('should create a new graph state with default name', () => {
      const state = createEmptyGraphState();
      
      expect(state.name).toBe('Untitled');
      expect(state.nodes).toEqual([]);
      expect(state.connections).toEqual([]);
      expect(state.id).toBeDefined();
      expect(state.createdAt).toBeDefined();
      expect(state.updatedAt).toBeDefined();
    });

    it('should create a graph state with custom name', () => {
      const state = createEmptyGraphState('My Graph');
      
      expect(state.name).toBe('My Graph');
    });

    it('should generate unique IDs', () => {
      const state1 = createEmptyGraphState();
      const state2 = createEmptyGraphState();
      
      expect(state1.id).not.toBe(state2.id);
    });
  });

  describe('createTabState', () => {
    it('should create a tab state with default name', () => {
      const tab = createTabState();
      
      expect(tab.name).toBe('New Tab');
      expect(tab.isActive).toBe(false);
      expect(tab.graphState).toBeDefined();
      expect(tab.id).toBeDefined();
    });

    it('should create a tab state with custom name', () => {
      const tab = createTabState('Custom Tab');
      
      expect(tab.name).toBe('Custom Tab');
      expect(tab.graphState.name).toBe('Custom Tab');
    });
  });

  describe('createInitialSessionState', () => {
    it('should create session with one active tab', () => {
      const session = createInitialSessionState();
      
      expect(session.tabs).toHaveLength(1);
      expect(session.tabs[0].isActive).toBe(true);
      expect(session.tabs[0].name).toBe('Tab 1');
      expect(session.activeTabId).toBe(session.tabs[0].id);
    });
  });
});

describe('Graph State Serialization', () => {
  it('should create valid JSON from graph state', () => {
    const state = createEmptyGraphState('Test Graph');
    const json = JSON.stringify(state);
    const parsed = JSON.parse(json);
    
    expect(parsed.name).toBe('Test Graph');
    expect(parsed.nodes).toEqual([]);
    expect(parsed.connections).toEqual([]);
  });

  it('should handle graph state with nodes', () => {
    const state = createEmptyGraphState();
    state.nodes.push({
      id: 'node-1',
      type: 'AudioInputNode',
      position: { x: 100, y: 200 },
      controls: { url: 'test.mp3' },
    });
    
    const json = JSON.stringify(state);
    const parsed = JSON.parse(json);
    
    expect(parsed.nodes).toHaveLength(1);
    expect(parsed.nodes[0].type).toBe('AudioInputNode');
    expect(parsed.nodes[0].position).toEqual({ x: 100, y: 200 });
  });

  it('should handle graph state with connections', () => {
    const state = createEmptyGraphState();
    state.connections.push({
      id: 'conn-1',
      source: 'node-1',
      sourceOutput: 'audio',
      target: 'node-2',
      targetInput: 'audio',
    });
    
    const json = JSON.stringify(state);
    const parsed = JSON.parse(json);
    
    expect(parsed.connections).toHaveLength(1);
    expect(parsed.connections[0].source).toBe('node-1');
  });
});
