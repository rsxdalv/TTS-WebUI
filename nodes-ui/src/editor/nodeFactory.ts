import { NodeEditor, ClassicPreset } from 'rete';
import { AreaPlugin } from 'rete-area-plugin';
import { Schemes, AreaExtra } from './createEditor';
import {
  BaseNode,
  AudioInputNode,
  AudioDisplayNode,
  SaveAudioNode,
  TextInputNode,
  NumberInputNode,
  OpenAITTSNode,
  GradioAPINode,
  CustomJSNode,
} from '../nodes';

// Node type registry
export const nodeTypes = {
  AudioInputNode,
  AudioDisplayNode,
  SaveAudioNode,
  TextInputNode,
  NumberInputNode,
  OpenAITTSNode,
  GradioAPINode,
  CustomJSNode,
} as const;

export type NodeTypeName = keyof typeof nodeTypes;

export function createNodeFactory(
  editor: NodeEditor<Schemes>,
  area: AreaPlugin<Schemes, AreaExtra>
) {
  // Create a node by type name
  async function createNode(
    typeName: NodeTypeName,
    position?: { x: number; y: number }
  ): Promise<ClassicPreset.Node> {
    const NodeClass = nodeTypes[typeName];
    if (!NodeClass) {
      throw new Error(`Unknown node type: ${typeName}`);
    }
    
    const node = new NodeClass();
    await editor.addNode(node as any);
    
    if (position) {
      await area.translate(node.id, position);
    }
    
    return node;
  }
  
  // Create a connection between nodes
  async function createConnection(
    sourceNode: ClassicPreset.Node,
    sourceOutput: string,
    targetNode: ClassicPreset.Node,
    targetInput: string
  ): Promise<ClassicPreset.Connection<ClassicPreset.Node, ClassicPreset.Node>> {
    const connection = new ClassicPreset.Connection(
      sourceNode,
      sourceOutput,
      targetNode,
      targetInput
    );
    await editor.addConnection(connection as any);
    return connection;
  }
  
  // Remove a node
  async function removeNode(nodeId: string): Promise<void> {
    await editor.removeNode(nodeId);
  }
  
  // Remove a connection
  async function removeConnection(connectionId: string): Promise<void> {
    await editor.removeConnection(connectionId);
  }
  
  // Clear all nodes and connections
  async function clear(): Promise<void> {
    await editor.clear();
  }
  
  // Get all available node types grouped by category
  function getNodeCategories(): Record<string, NodeTypeName[]> {
    const categories: Record<string, NodeTypeName[]> = {};
    
    for (const [name, NodeClass] of Object.entries(nodeTypes)) {
      const instance = new NodeClass();
      const category = instance.category;
      
      if (!categories[category]) {
        categories[category] = [];
      }
      categories[category].push(name as NodeTypeName);
    }
    
    return categories;
  }
  
  return {
    createNode,
    createConnection,
    removeNode,
    removeConnection,
    clear,
    getNodeCategories,
    nodeTypes,
  };
}

export type NodeFactory = ReturnType<typeof createNodeFactory>;
