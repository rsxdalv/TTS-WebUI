import { OpenAITTSNode } from '../nodes/OpenAITTSNode';
import { GradioAPINode } from '../nodes/GradioAPINode';

describe('OpenAITTSNode', () => {
  it('should create with default values', () => {
    const node = new OpenAITTSNode();
    expect(node.label).toBe('OpenAI TTS');
    expect(node.category).toBe('TTS');
    expect(node.getControlValue('model')).toBe('tts-1');
    expect(node.getControlValue('voice')).toBe('alloy');
    expect(node.getControlValue('speed')).toBe(1.0);
  });

  it('should list available voices', () => {
    const node = new OpenAITTSNode();
    const voices = node.getVoices();
    
    expect(voices).toContain('alloy');
    expect(voices).toContain('echo');
    expect(voices).toContain('nova');
  });

  it('should list available models', () => {
    const node = new OpenAITTSNode();
    const models = node.getModels();
    
    expect(models).toContain('tts-1');
    expect(models).toContain('tts-1-hd');
  });

  it('should return null audio when no text provided', async () => {
    const node = new OpenAITTSNode();
    node.setControlValue('apiKey', 'test-key');
    
    const result = await node.execute({});
    
    expect(result.audio).toBeNull();
  });

  it('should return null audio when no API key provided', async () => {
    const node = new OpenAITTSNode();
    
    const result = await node.execute({ text: 'Hello world' });
    
    expect(result.audio).toBeNull();
  });

  it('should serialize control values', () => {
    const node = new OpenAITTSNode();
    node.setControlValue('model', 'tts-1-hd');
    node.setControlValue('voice', 'nova');
    
    const serialized = node.serialize();
    
    expect(serialized.type).toBe('OpenAITTSNode');
    expect(serialized.controls.model).toBe('tts-1-hd');
    expect(serialized.controls.voice).toBe('nova');
  });
});

describe('GradioAPINode', () => {
  it('should create with default values', () => {
    const node = new GradioAPINode();
    expect(node.label).toBe('Gradio API');
    expect(node.category).toBe('API');
    expect(node.getControlValue('backendUrl')).toBe('http://localhost:7860');
  });

  it('should set endpoint', () => {
    const node = new GradioAPINode();
    node.setEndpoint('mms');
    
    expect(node.getControlValue('endpoint')).toBe('mms');
  });

  it('should set backend URL', () => {
    const node = new GradioAPINode();
    node.setBackendUrl('http://custom:8080');
    
    expect(node.getControlValue('backendUrl')).toBe('http://custom:8080');
  });

  it('should return null output when no endpoint set', async () => {
    const node = new GradioAPINode();
    
    const result = await node.execute({ input: 'test' });
    
    expect(result.output).toBeNull();
  });

  it('should serialize control values', () => {
    const node = new GradioAPINode();
    node.setEndpoint('bark');
    node.setBackendUrl('http://test:7860');
    
    const serialized = node.serialize();
    
    expect(serialized.controls.endpoint).toBe('bark');
    expect(serialized.controls.backendUrl).toBe('http://test:7860');
  });
});
