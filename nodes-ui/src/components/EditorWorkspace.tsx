import React, { useState, useCallback, useEffect, useRef } from 'react';
import { NodeEditor } from './NodeEditor';
import { TabBar } from './TabBar';
import { Button } from './ui/button';
import { SessionState, GraphState } from '@/types';
import { EditorInstance, NodeFactory } from '@/editor';
import {
  createInitialSessionState,
  saveSessionToStorage,
  loadSessionFromStorage,
  serializeGraph,
  deserializeGraph,
  downloadGraphAsJson,
} from '@/editor/graphState';
import { Play, Square, Trash2 } from 'lucide-react';

export function EditorWorkspace() {
  const [session, setSession] = useState<SessionState>(createInitialSessionState());
  const [isLoaded, setIsLoaded] = useState(false);
  const [isExecuting, setIsExecuting] = useState(false);
  
  const editorRef = useRef<EditorInstance | null>(null);
  const factoryRef = useRef<NodeFactory | null>(null);
  
  // Load session from localStorage on mount
  useEffect(() => {
    const stored = loadSessionFromStorage();
    if (stored) {
      setSession(stored);
    }
    setIsLoaded(true);
  }, []);
  
  // Save session to localStorage on change
  useEffect(() => {
    if (isLoaded) {
      saveSessionToStorage(session);
    }
  }, [session, isLoaded]);
  
  const handleEditorReady = useCallback(async (editor: EditorInstance, factory: NodeFactory) => {
    editorRef.current = editor;
    factoryRef.current = factory;
    
    // Load the current tab's graph state
    const activeTab = session.tabs.find(t => t.isActive);
    if (activeTab && activeTab.graphState.nodes.length > 0) {
      await deserializeGraph(activeTab.graphState, editor.editor, editor.area, factory);
    }
  }, [session.tabs]);
  
  const handleTabChange = useCallback(async (tabId: string) => {
    // Save current graph state before switching
    if (editorRef.current && factoryRef.current) {
      const currentTab = session.tabs.find(t => t.isActive);
      if (currentTab) {
        const graphState = await serializeGraph(
          editorRef.current.editor,
          editorRef.current.area,
          currentTab.graphState
        );
        
        setSession(prev => ({
          ...prev,
          tabs: prev.tabs.map(t =>
            t.id === currentTab.id ? { ...t, graphState } : t
          ),
        }));
      }
      
      // Load the new tab's graph
      const newTab = session.tabs.find(t => t.id === tabId);
      if (newTab) {
        await deserializeGraph(
          newTab.graphState,
          editorRef.current.editor,
          editorRef.current.area,
          factoryRef.current
        );
      }
    }
  }, [session.tabs]);
  
  const handleSaveGraph = useCallback(async () => {
    if (!editorRef.current) return;
    
    const activeTab = session.tabs.find(t => t.isActive);
    if (!activeTab) return;
    
    const graphState = await serializeGraph(
      editorRef.current.editor,
      editorRef.current.area,
      activeTab.graphState
    );
    
    downloadGraphAsJson(graphState);
  }, [session.tabs]);
  
  const handleClearGraph = useCallback(async () => {
    if (!factoryRef.current) return;
    await factoryRef.current.clear();
  }, []);
  
  const handleExecuteGraph = useCallback(async () => {
    if (!editorRef.current || isExecuting) return;
    
    setIsExecuting(true);
    
    try {
      const nodes = editorRef.current.editor.getNodes();
      const connections = editorRef.current.editor.getConnections();
      
      // Simple topological execution - find nodes with no inputs and execute
      const nodeOutputs = new Map<string, Record<string, any>>();
      
      // Build dependency graph
      const nodeDependencies = new Map<string, Set<string>>();
      for (const node of nodes) {
        nodeDependencies.set(node.id, new Set());
      }
      
      for (const conn of connections) {
        const deps = nodeDependencies.get(conn.target);
        if (deps) {
          deps.add(conn.source);
        }
      }
      
      // Execute nodes in order
      const executed = new Set<string>();
      const maxIterations = nodes.length * 2;
      let iterations = 0;
      
      while (executed.size < nodes.length && iterations < maxIterations) {
        iterations++;
        
        for (const node of nodes) {
          if (executed.has(node.id)) continue;
          
          const deps = nodeDependencies.get(node.id);
          const allDepsExecuted = !deps || [...deps].every(d => executed.has(d));
          
          if (allDepsExecuted) {
            // Gather inputs from connected nodes
            const inputs: Record<string, any> = {};
            for (const conn of connections) {
              if (conn.target === node.id) {
                const sourceOutput = nodeOutputs.get(conn.source);
                if (sourceOutput) {
                  inputs[conn.targetInput] = sourceOutput[conn.sourceOutput];
                }
              }
            }
            
            // Execute the node
            try {
              const outputs = await (node as any).execute(inputs);
              nodeOutputs.set(node.id, outputs);
            } catch (error) {
              console.error(`Failed to execute node ${node.label}:`, error);
            }
            
            executed.add(node.id);
          }
        }
      }
      
      console.log('Graph execution complete');
    } finally {
      setIsExecuting(false);
    }
  }, [isExecuting]);
  
  if (!isLoaded) {
    return (
      <div className="flex items-center justify-center h-screen bg-background">
        <div className="text-muted-foreground">Loading...</div>
      </div>
    );
  }
  
  return (
    <div className="flex flex-col h-screen bg-background">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 bg-card border-b border-border">
        <h1 className="text-lg font-semibold">TTS-WebUI Nodes</h1>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleExecuteGraph}
            disabled={isExecuting}
          >
            {isExecuting ? (
              <>
                <Square className="w-4 h-4 mr-2" />
                Running...
              </>
            ) : (
              <>
                <Play className="w-4 h-4 mr-2" />
                Execute
              </>
            )}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleClearGraph}
          >
            <Trash2 className="w-4 h-4 mr-2" />
            Clear
          </Button>
        </div>
      </div>
      
      {/* Tab Bar */}
      <TabBar
        session={session}
        onSessionChange={setSession}
        onTabChange={handleTabChange}
        onSaveCurrentGraph={handleSaveGraph}
      />
      
      {/* Editor */}
      <div className="flex-1 overflow-hidden">
        <NodeEditor onEditorReady={handleEditorReady} />
      </div>
      
      {/* Status Bar */}
      <div className="flex items-center justify-between px-4 py-1 text-xs text-muted-foreground bg-card border-t border-border">
        <span>Right-click to add nodes</span>
        <span>Nodes: {editorRef.current?.editor.getNodes().length || 0}</span>
      </div>
    </div>
  );
}
