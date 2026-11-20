import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { textSocket } from '../types';

export class TextInputNode extends BaseNode {
  readonly category = 'Input';
  
  width = 240;
  height = 150;
  
  constructor() {
    super('Text Input');
    
    // Output for text data
    this.addOutput('text', new ClassicPreset.Output(textSocket, 'Text'));
    
    // Control for text input
    this.addControl('text', new ClassicPreset.InputControl('text', {
      initial: '',
    }));
  }
  
  async execute(_inputs: Record<string, any>): Promise<Record<string, string>> {
    const text = this.getControlValue<string>('text') || '';
    return { text };
  }
}
