import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { numberSocket } from '../types';

export class NumberInputNode extends BaseNode {
  readonly category = 'Input';
  
  width = 200;
  height = 150;
  
  constructor() {
    super('Number Input');
    
    // Output for number data
    this.addOutput('number', new ClassicPreset.Output(numberSocket, 'Number'));
    
    // Control for number input
    this.addControl('number', new ClassicPreset.InputControl('number', {
      initial: 0,
    }));
  }
  
  async execute(_inputs: Record<string, any>): Promise<Record<string, number>> {
    const value = this.getControlValue<number>('number') || 0;
    return { number: value };
  }
}
