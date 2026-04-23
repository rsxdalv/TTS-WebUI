import React, { useState, useCallback } from 'react';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Plus, X, Save, Upload, Trash2 } from 'lucide-react';
import { TabState, SessionState } from '@/types';
import {
  createTabState,
  downloadGraphAsJson,
  loadGraphFromJson,
} from '@/editor/graphState';

interface TabBarProps {
  session: SessionState;
  onSessionChange: (session: SessionState) => void;
  onTabChange: (tabId: string) => void;
  onSaveCurrentGraph: () => void;
}

export function TabBar({
  session,
  onSessionChange,
  onTabChange,
  onSaveCurrentGraph,
}: TabBarProps) {
  const [editingTabId, setEditingTabId] = useState<string | null>(null);
  const [editingName, setEditingName] = useState('');
  
  const handleAddTab = useCallback(() => {
    const newTab = createTabState(`Tab ${session.tabs.length + 1}`);
    const updatedTabs = session.tabs.map(t => ({ ...t, isActive: false }));
    updatedTabs.push({ ...newTab, isActive: true });
    
    onSessionChange({
      tabs: updatedTabs,
      activeTabId: newTab.id,
    });
    onTabChange(newTab.id);
  }, [session, onSessionChange, onTabChange]);
  
  const handleCloseTab = useCallback((tabId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    
    if (session.tabs.length === 1) {
      // Don't close the last tab, just reset it
      const resetTab = createTabState('Tab 1');
      resetTab.isActive = true;
      onSessionChange({
        tabs: [resetTab],
        activeTabId: resetTab.id,
      });
      onTabChange(resetTab.id);
      return;
    }
    
    const tabIndex = session.tabs.findIndex(t => t.id === tabId);
    const newTabs = session.tabs.filter(t => t.id !== tabId);
    
    // If closing active tab, activate the previous or next one
    if (tabId === session.activeTabId) {
      const newActiveIndex = Math.max(0, tabIndex - 1);
      newTabs[newActiveIndex].isActive = true;
      onSessionChange({
        tabs: newTabs,
        activeTabId: newTabs[newActiveIndex].id,
      });
      onTabChange(newTabs[newActiveIndex].id);
    } else {
      onSessionChange({
        ...session,
        tabs: newTabs,
      });
    }
  }, [session, onSessionChange, onTabChange]);
  
  const handleSelectTab = useCallback((tabId: string) => {
    if (tabId === session.activeTabId) return;
    
    const updatedTabs = session.tabs.map(t => ({
      ...t,
      isActive: t.id === tabId,
    }));
    
    onSessionChange({
      tabs: updatedTabs,
      activeTabId: tabId,
    });
    onTabChange(tabId);
  }, [session, onSessionChange, onTabChange]);
  
  const handleStartRename = useCallback((tab: TabState, e: React.MouseEvent) => {
    e.stopPropagation();
    setEditingTabId(tab.id);
    setEditingName(tab.name);
  }, []);
  
  const handleFinishRename = useCallback(() => {
    if (!editingTabId) return;
    
    const updatedTabs = session.tabs.map(t =>
      t.id === editingTabId ? { ...t, name: editingName || t.name } : t
    );
    
    onSessionChange({
      ...session,
      tabs: updatedTabs,
    });
    setEditingTabId(null);
    setEditingName('');
  }, [editingTabId, editingName, session, onSessionChange]);
  
  const handleLoadFile = useCallback(async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    
    try {
      const graphState = await loadGraphFromJson(file);
      const newTab = createTabState(graphState.name);
      newTab.graphState = graphState;
      newTab.isActive = true;
      
      const updatedTabs = session.tabs.map(t => ({ ...t, isActive: false }));
      updatedTabs.push(newTab);
      
      onSessionChange({
        tabs: updatedTabs,
        activeTabId: newTab.id,
      });
      onTabChange(newTab.id);
    } catch (error) {
      console.error('Failed to load graph:', error);
    }
    
    // Reset file input
    e.target.value = '';
  }, [session, onSessionChange, onTabChange]);
  
  return (
    <div className="flex items-center gap-1 p-1 bg-muted border-b border-border">
      <div className="flex items-center gap-1 flex-1 overflow-x-auto">
        {session.tabs.map(tab => (
          <div
            key={tab.id}
            onClick={() => handleSelectTab(tab.id)}
            className={`
              flex items-center gap-2 px-3 py-1.5 rounded-t-md cursor-pointer
              transition-colors min-w-[100px] max-w-[200px]
              ${tab.isActive 
                ? 'bg-background text-foreground' 
                : 'text-muted-foreground hover:bg-background/50'
              }
            `}
          >
            {editingTabId === tab.id ? (
              <Input
                value={editingName}
                onChange={(e) => setEditingName(e.target.value)}
                onBlur={handleFinishRename}
                onKeyDown={(e) => e.key === 'Enter' && handleFinishRename()}
                className="h-6 px-1 py-0 text-sm"
                autoFocus
                onClick={(e) => e.stopPropagation()}
              />
            ) : (
              <span
                className="truncate text-sm"
                onDoubleClick={(e) => handleStartRename(tab, e)}
              >
                {tab.name}
              </span>
            )}
            <button
              onClick={(e) => handleCloseTab(tab.id, e)}
              className="p-0.5 rounded hover:bg-muted"
            >
              <X className="w-3 h-3" />
            </button>
          </div>
        ))}
      </div>
      
      <div className="flex items-center gap-1 px-2 border-l border-border">
        <Button
          variant="ghost"
          size="icon"
          onClick={handleAddTab}
          title="New Tab"
        >
          <Plus className="w-4 h-4" />
        </Button>
        
        <Button
          variant="ghost"
          size="icon"
          onClick={onSaveCurrentGraph}
          title="Save Graph"
        >
          <Save className="w-4 h-4" />
        </Button>
        
        <label title="Load Graph">
          <Button
            variant="ghost"
            size="icon"
            asChild
          >
            <span>
              <Upload className="w-4 h-4" />
            </span>
          </Button>
          <input
            type="file"
            accept=".json"
            className="hidden"
            onChange={handleLoadFile}
          />
        </label>
      </div>
    </div>
  );
}
