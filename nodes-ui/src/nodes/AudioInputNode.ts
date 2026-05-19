import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { audioSocket, AudioData } from '../types';

export class AudioInputNode extends BaseNode {
  readonly category = 'Audio';
  
  width = 240;
  height = 200;
  
  constructor() {
    super('Audio Input');
    
    // Output for the audio data
    this.addOutput('audio', new ClassicPreset.Output(audioSocket, 'Audio'));
    
    // Control for file URL or upload status
    this.addControl('url', new ClassicPreset.InputControl('text', {
      initial: '',
    }));
  }
  
  async execute(_inputs: Record<string, any>): Promise<Record<string, AudioData>> {
    const url = this.getControlValue<string>('url') || '';
    
    return {
      audio: {
        url,
        name: url.split('/').pop() || 'audio',
      },
    };
  }
  
  // Set audio from file upload
  setAudioFile(file: File): void {
    const url = URL.createObjectURL(file);
    this.setControlValue('url', url);
    this.setControlValue('fileName', file.name);
  }
  
  // Set audio from URL
  setAudioUrl(url: string): void {
    this.setControlValue('url', url);
  }
}
