import { AudioInputNode } from '../nodes/AudioInputNode';
import { AudioDisplayNode } from '../nodes/AudioDisplayNode';
import { SaveAudioNode } from '../nodes/SaveAudioNode';

describe('AudioInputNode', () => {
  it('should create with default values', () => {
    const node = new AudioInputNode();
    expect(node.label).toBe('Audio Input');
    expect(node.category).toBe('Audio');
  });

  it('should execute and return audio data', async () => {
    const node = new AudioInputNode();
    node.setControlValue('url', 'http://example.com/audio.mp3');
    
    const result = await node.execute({});
    
    expect(result.audio).toBeDefined();
    expect(result.audio.url).toBe('http://example.com/audio.mp3');
  });

  it('should serialize control values', () => {
    const node = new AudioInputNode();
    node.setControlValue('url', 'test.mp3');
    
    const serialized = node.serialize();
    
    expect(serialized.type).toBe('AudioInputNode');
    expect(serialized.controls.url).toBe('test.mp3');
  });
});

describe('AudioDisplayNode', () => {
  it('should create with default values', () => {
    const node = new AudioDisplayNode();
    expect(node.label).toBe('Audio Display');
    expect(node.category).toBe('Audio');
  });

  it('should pass through audio data', async () => {
    const node = new AudioDisplayNode();
    const inputAudio = { url: 'http://example.com/audio.mp3', name: 'test' };
    
    const result = await node.execute({ audio: inputAudio });
    
    expect(result.audio).toEqual(inputAudio);
    expect(node.getAudioData()).toEqual(inputAudio);
  });

  it('should handle missing audio input', async () => {
    const node = new AudioDisplayNode();
    
    const result = await node.execute({});
    
    expect(result.audio).toBeNull();
  });
});

describe('SaveAudioNode', () => {
  it('should create with default filename', () => {
    const node = new SaveAudioNode();
    expect(node.label).toBe('Save Audio');
    expect(node.getControlValue('filename')).toBe('output.wav');
  });

  it('should return error when no audio input', async () => {
    const node = new SaveAudioNode();
    
    const result = await node.execute({});
    
    expect(result.success).toBe(false);
    expect(result.error).toBe('No audio input');
  });
});
