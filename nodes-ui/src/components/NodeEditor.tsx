import React, { useRef, useEffect, useState, useCallback } from 'react';
import { createEditor, EditorInstance } from '@/editor/createEditor';
import { createNodeFactory, NodeFactory } from '@/editor/nodeFactory';

interface NodeEditorProps {
  onEditorReady?: (editor: EditorInstance, factory: NodeFactory) => void;
  className?: string;
}

export function NodeEditor({ onEditorReady, className }: NodeEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [editorInstance, setEditorInstance] = useState<EditorInstance | null>(null);
  const [factory, setFactory] = useState<NodeFactory | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  useEffect(() => {
    let mounted = true;
    let instance: EditorInstance | null = null;
    
    async function initEditor() {
      if (!containerRef.current) return;
      
      try {
        // Create a temporary factory to pass to createEditor
        // We'll update it after the editor is created
        const tempEditor = await import('@/editor/createEditor');
        const nodeFactory = await import('@/editor/nodeFactory');
        
        // Create editor first with a placeholder factory
        const placeholderFactory = {
          createNode: async () => { throw new Error('Not ready'); },
        } as any;
        
        instance = await tempEditor.createEditor(containerRef.current, placeholderFactory);
        
        if (!mounted) {
          instance.destroy();
          return;
        }
        
        // Now create the real factory with the editor
        const realFactory = nodeFactory.createNodeFactory(instance.editor, instance.area);
        
        setEditorInstance(instance);
        setFactory(realFactory);
        setIsLoading(false);
        
        if (onEditorReady) {
          onEditorReady(instance, realFactory);
        }
      } catch (err) {
        console.error('Failed to initialize editor:', err);
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Failed to initialize editor');
          setIsLoading(false);
        }
      }
    }
    
    initEditor();
    
    return () => {
      mounted = false;
      if (instance) {
        instance.destroy();
      }
    };
  }, [onEditorReady]);
  
  if (error) {
    return (
      <div className={`flex items-center justify-center h-full ${className || ''}`}>
        <div className="text-destructive">
          Error: {error}
        </div>
      </div>
    );
  }
  
  return (
    <div className={`relative w-full h-full ${className || ''}`} style={{ minHeight: '400px' }}>
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/80 z-10">
          <div className="text-muted-foreground">Loading editor...</div>
        </div>
      )}
      <div 
        ref={containerRef} 
        className="absolute inset-0"
        style={{ 
          minHeight: '400px',
          backgroundColor: 'hsl(var(--background))',
        }}
      />
    </div>
  );
}
