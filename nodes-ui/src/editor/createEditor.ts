import { NodeEditor, GetSchemes, ClassicPreset } from 'rete';
import { AreaPlugin, AreaExtensions } from 'rete-area-plugin';
import { ConnectionPlugin, Presets as ConnectionPresets } from 'rete-connection-plugin';
import { ReactPlugin, Presets, ReactArea2D } from 'rete-react-plugin';
import { ContextMenuPlugin, ContextMenuExtra, Presets as ContextMenuPresets } from 'rete-context-menu-plugin';
import { createRoot } from 'react-dom/client';

// Use ClassicPreset.Node directly for compatibility
type Node = ClassicPreset.Node;
type Connection = ClassicPreset.Connection<Node, Node>;

// Define the schemes for type safety
export type Schemes = GetSchemes<Node, Connection>;

export type AreaExtra = ReactArea2D<Schemes> | ContextMenuExtra;

export interface EditorInstance {
  editor: NodeEditor<Schemes>;
  area: AreaPlugin<Schemes, AreaExtra>;
  connection: ConnectionPlugin<Schemes, AreaExtra>;
  render: ReactPlugin<Schemes, AreaExtra>;
  contextMenu: ContextMenuPlugin<Schemes>;
  destroy: () => void;
}

export async function createEditor(
  container: HTMLElement,
  nodeFactory: ReturnType<typeof import('./nodeFactory').createNodeFactory>
): Promise<EditorInstance> {
  const editor = new NodeEditor<Schemes>();
  const area = new AreaPlugin<Schemes, AreaExtra>(container);
  const connection = new ConnectionPlugin<Schemes, AreaExtra>();
  const render = new ReactPlugin<Schemes, AreaExtra>({ createRoot });
  const contextMenu = new ContextMenuPlugin<Schemes>({
    items: ContextMenuPresets.classic.setup([
      ['Audio', [
        ['Audio Input', () => nodeFactory.createNode('AudioInputNode')],
        ['Audio Display', () => nodeFactory.createNode('AudioDisplayNode')],
        ['Save Audio', () => nodeFactory.createNode('SaveAudioNode')],
      ]],
      ['TTS', [
        ['OpenAI TTS', () => nodeFactory.createNode('OpenAITTSNode')],
        ['Text Input', () => nodeFactory.createNode('TextInputNode')],
      ]],
      ['Processing', [
        ['Number Input', () => nodeFactory.createNode('NumberInputNode')],
        ['Custom JS', () => nodeFactory.createNode('CustomJSNode')],
      ]],
      ['API', [
        ['Gradio API', () => nodeFactory.createNode('GradioAPINode')],
      ]],
    ]),
  });

  // Apply the presets for React rendering
  render.addPreset(Presets.classic.setup());
  render.addPreset(Presets.contextMenu.setup());
  connection.addPreset(ConnectionPresets.classic.setup());

  // Use plugins
  editor.use(area);
  area.use(connection);
  area.use(render);
  area.use(contextMenu);

  // Enable zooming and panning
  AreaExtensions.zoomAt(area, editor.getNodes());
  AreaExtensions.simpleNodesOrder(area);
  AreaExtensions.selectableNodes(area, AreaExtensions.selector(), {
    accumulating: AreaExtensions.accumulateOnCtrl(),
  });

  const destroy = () => {
    area.destroy();
  };

  return {
    editor,
    area,
    connection,
    render,
    contextMenu,
    destroy,
  };
}
