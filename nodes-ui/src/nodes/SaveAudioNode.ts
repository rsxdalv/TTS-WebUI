import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { audioSocket, AudioData } from '../types';

export class SaveAudioNode extends BaseNode {
  readonly category = 'Audio';
  
  width = 240;
  height = 180;
  
  constructor() {
    super('Save Audio');
    
    // Input for audio data
    this.addInput('audio', new ClassicPreset.Input(audioSocket, 'Audio'));
    
    // Control for filename
    this.addControl('filename', new ClassicPreset.InputControl('text', {
      initial: 'output.wav',
    }));
  }
  
  async execute(inputs: Record<string, any>): Promise<Record<string, any>> {
    const audio = inputs.audio as AudioData | undefined;
    const filename = this.getControlValue<string>('filename') || 'output.wav';
    
    if (audio && audio.url) {
      // Trigger download
      await this.downloadAudio(audio, filename);
      return { success: true, filename };
    }
    
    return { success: false, error: 'No audio input' };
  }
  
  private async downloadAudio(audio: AudioData, filename: string): Promise<void> {
    try {
      // If we have a blob, use it directly
      if (audio.blob) {
        const url = URL.createObjectURL(audio.blob);
        this.triggerDownload(url, filename);
        URL.revokeObjectURL(url);
        return;
      }
      
      // Otherwise, fetch the URL and download
      const response = await fetch(audio.url);
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      this.triggerDownload(url, filename);
      URL.revokeObjectURL(url);
    } catch (error) {
      console.error('Failed to save audio:', error);
      throw error;
    }
  }
  
  private triggerDownload(url: string, filename: string): void {
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }
}
