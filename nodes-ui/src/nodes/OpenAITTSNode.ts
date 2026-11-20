import { ClassicPreset } from 'rete';
import { BaseNode } from './BaseNode';
import { textSocket, audioSocket, AudioData } from '../types';

// OpenAI TTS voices
const OPENAI_VOICES = ['alloy', 'echo', 'fable', 'onyx', 'nova', 'shimmer'] as const;
const OPENAI_MODELS = ['tts-1', 'tts-1-hd'] as const;

export class OpenAITTSNode extends BaseNode {
  readonly category = 'TTS';
  
  width = 280;
  height = 300;
  
  constructor() {
    super('OpenAI TTS');
    
    // Input for text
    this.addInput('text', new ClassicPreset.Input(textSocket, 'Text'));
    
    // Output for audio
    this.addOutput('audio', new ClassicPreset.Output(audioSocket, 'Audio'));
    
    // Controls
    this.addControl('apiKey', new ClassicPreset.InputControl('text', {
      initial: '',
    }));
    
    this.addControl('model', new ClassicPreset.InputControl('text', {
      initial: 'tts-1',
    }));
    
    this.addControl('voice', new ClassicPreset.InputControl('text', {
      initial: 'alloy',
    }));
    
    this.addControl('speed', new ClassicPreset.InputControl('number', {
      initial: 1.0,
    }));
    
    // Store additional metadata
    this.controlValues.voices = OPENAI_VOICES;
    this.controlValues.models = OPENAI_MODELS;
  }
  
  async execute(inputs: Record<string, any>): Promise<Record<string, AudioData | null>> {
    const textInput = inputs.text as string | undefined;
    const text = textInput || this.getControlValue<string>('textOverride') || '';
    const apiKey = this.getControlValue<string>('apiKey') || '';
    const model = this.getControlValue<string>('model') || 'tts-1';
    const voice = this.getControlValue<string>('voice') || 'alloy';
    const speed = this.getControlValue<number>('speed') || 1.0;
    
    if (!text) {
      return { audio: null };
    }
    
    if (!apiKey) {
      // Return null without exposing implementation details in console
      return { audio: null };
    }
    
    try {
      const response = await fetch('https://api.openai.com/v1/audio/speech', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${apiKey}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          model,
          input: text,
          voice,
          speed,
        }),
      });
      
      if (!response.ok) {
        throw new Error(`OpenAI API error: ${response.statusText}`);
      }
      
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      
      return {
        audio: {
          url,
          name: 'openai-tts-output.mp3',
          blob,
        },
      };
    } catch (error) {
      console.error('OpenAI TTS error:', error);
      return { audio: null };
    }
  }
  
  // Get available voices
  getVoices(): readonly string[] {
    return OPENAI_VOICES;
  }
  
  // Get available models
  getModels(): readonly string[] {
    return OPENAI_MODELS;
  }
}
