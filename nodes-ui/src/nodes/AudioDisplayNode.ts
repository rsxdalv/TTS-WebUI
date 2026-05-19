import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { audioSocket, AudioData } from '../types';

export class AudioDisplayNode extends BaseNode {
  readonly category = 'Audio';
  
  width = 300;
  height = 200;
  
  // Store the current audio data
  private audioData: AudioData | null = null;
  
  constructor() {
    super('Audio Display');
    
    // Input for audio data
    this.addInput('audio', new ClassicPreset.Input(audioSocket, 'Audio'));
    
    // Output to pass through the audio
    this.addOutput('audio', new ClassicPreset.Output(audioSocket, 'Audio'));
  }
  
  async execute(inputs: Record<string, any>): Promise<Record<string, AudioData | null>> {
    const audio = inputs.audio as AudioData | undefined;
    this.audioData = audio || null;
    
    return {
      audio: this.audioData,
    };
  }
  
  // Get the current audio data for display
  getAudioData(): AudioData | null {
    return this.audioData;
  }
  
  // Set audio data directly (for testing or external updates)
  setAudioData(data: AudioData | null): void {
    this.audioData = data;
  }
}
