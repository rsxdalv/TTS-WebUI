import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { audioSocket, textSocket, AudioData } from '../types';

// Validate endpoint to prevent path traversal
function isValidEndpoint(endpoint: string): boolean {
  return /^[a-zA-Z0-9_\-\/]+$/.test(endpoint) && !endpoint.includes('..');
}

export class GradioAPINode extends BaseNode {
  readonly category = 'API';
  
  width = 300;
  height = 280;
  
  constructor() {
    super('Gradio API');
    
    // Dynamic inputs/outputs - we'll use text as default
    this.addInput('input', new ClassicPreset.Input(textSocket, 'Input'));
    this.addOutput('output', new ClassicPreset.Output(audioSocket, 'Output'));
    
    // Controls
    this.addControl('endpoint', new ClassicPreset.InputControl('text', {
      initial: '',
    }));
    
    this.addControl('backendUrl', new ClassicPreset.InputControl('text', {
      initial: 'http://localhost:7860',
    }));
  }
  
  async execute(inputs: Record<string, any>): Promise<Record<string, AudioData | null>> {
    const input = inputs.input;
    const endpoint = this.getControlValue<string>('endpoint') || '';
    
    if (!endpoint) {
      console.error('Endpoint is required');
      return { output: null };
    }
    
    if (!isValidEndpoint(endpoint)) {
      console.error('Invalid endpoint format');
      return { output: null };
    }
    
    try {
      // Call the Gradio API through our Next.js proxy
      const encodedEndpoint = encodeURIComponent(endpoint).replace(/%2F/g, '/');
      const response = await fetch(`/api/gradio?endpoint=${encodedEndpoint}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(input),
      });
      
      if (!response.ok) {
        throw new Error(`Gradio API error: ${response.statusText}`);
      }
      
      const result = await response.json();
      
      // Extract audio from result
      const audio = result.audio || result.data?.[0];
      
      if (audio && audio.url) {
        return {
          output: {
            url: audio.url,
            name: audio.orig_name || 'gradio-output',
          },
        };
      }
      
      return { output: null };
    } catch (error) {
      console.error('Gradio API error:', error);
      return { output: null };
    }
  }
  
  // Set the endpoint
  setEndpoint(endpoint: string): void {
    this.setControlValue('endpoint', endpoint);
  }
  
  // Set the backend URL
  setBackendUrl(url: string): void {
    this.setControlValue('backendUrl', url);
  }
}
