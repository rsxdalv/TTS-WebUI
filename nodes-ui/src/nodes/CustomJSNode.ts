import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { textSocket, numberSocket, audioSocket } from '../types';

/**
 * CustomJSNode allows executing user-defined JavaScript code.
 * 
 * SECURITY WARNING: This node executes arbitrary JavaScript code using the Function constructor.
 * It should only be used in trusted environments where the user has full control over the code.
 * The execution is not sandboxed and has access to the global scope.
 * 
 * For production use with untrusted input, consider using a Web Worker with restricted
 * permissions or a proper JavaScript sandboxing library.
 */
export class CustomJSNode extends BaseNode {
  readonly category = 'Processing';
  
  width = 320;
  height = 350;
  
  constructor() {
    super('Custom JS');
    
    // Generic inputs
    this.addInput('input1', new ClassicPreset.Input(textSocket, 'Input 1'));
    this.addInput('input2', new ClassicPreset.Input(numberSocket, 'Input 2'));
    
    // Generic outputs
    this.addOutput('output', new ClassicPreset.Output(textSocket, 'Output'));
    
    // Control for the JavaScript code
    this.addControl('code', new ClassicPreset.InputControl('text', {
      initial: `// Custom JavaScript code
// Available variables: input1, input2
// Return an object with output values
// WARNING: This executes in the browser context

return {
  output: input1 + " " + input2
};`,
    }));
  }
  
  async execute(inputs: Record<string, any>): Promise<Record<string, any>> {
    const code = this.getControlValue<string>('code') || '';
    const input1 = inputs.input1 ?? '';
    const input2 = inputs.input2 ?? 0;
    
    try {
      // Note: This uses the Function constructor which is not sandboxed
      // For untrusted code, consider using a Web Worker or sandbox library
      const fn = new Function('input1', 'input2', code);
      const result = fn(input1, input2);
      
      return result || { output: null };
    } catch (error) {
      console.error('Custom JS execution error:', error);
      return { output: null, error: String(error) };
    }
  }
  
  // Set the code
  setCode(code: string): void {
    this.setControlValue('code', code);
  }
  
  // Get the current code
  getCode(): string {
    return this.getControlValue<string>('code') || '';
  }
}

// Type for custom node configuration loaded from external files
export interface CustomNodeDefinition {
  id: string;
  name: string;
  category: string;
  description?: string;
  inputs: Array<{
    key: string;
    label: string;
    socket: 'audio' | 'text' | 'number';
  }>;
  outputs: Array<{
    key: string;
    label: string;
    socket: 'audio' | 'text' | 'number';
  }>;
  code: string;
}

// Factory function to create a custom node from definition
export function createCustomNodeFromDefinition(definition: CustomNodeDefinition): typeof BaseNode {
  const getSocket = (type: 'audio' | 'text' | 'number') => {
    switch (type) {
      case 'audio': return audioSocket;
      case 'text': return textSocket;
      case 'number': return numberSocket;
      default: return textSocket;
    }
  };
  
  return class DynamicCustomNode extends BaseNode {
    readonly category = definition.category;
    
    constructor() {
      super(definition.name);
      
      // Add inputs
      for (const input of definition.inputs) {
        this.addInput(input.key, new ClassicPreset.Input(getSocket(input.socket), input.label));
      }
      
      // Add outputs
      for (const output of definition.outputs) {
        this.addOutput(output.key, new ClassicPreset.Output(getSocket(output.socket), output.label));
      }
    }
    
    async execute(inputs: Record<string, any>): Promise<Record<string, any>> {
      try {
        // Build argument names and values from inputs
        const argNames = definition.inputs.map(i => i.key);
        const argValues = argNames.map(name => inputs[name]);
        
        // Create and execute the function
        const fn = new Function(...argNames, definition.code);
        const result = fn(...argValues);
        
        return result || {};
      } catch (error) {
        console.error(`Custom node "${definition.name}" execution error:`, error);
        return {};
      }
    }
  };
}
