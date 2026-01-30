import { TextInputNode } from '../nodes/TextInputNode';
import { NumberInputNode } from '../nodes/NumberInputNode';
import { CustomJSNode } from '../nodes/CustomJSNode';

describe('TextInputNode', () => {
  it('should create with default values', () => {
    const node = new TextInputNode();
    expect(node.label).toBe('Text Input');
    expect(node.category).toBe('Input');
  });

  it('should execute and return text', async () => {
    const node = new TextInputNode();
    node.setControlValue('text', 'Hello World');
    
    const result = await node.execute({});
    
    expect(result.text).toBe('Hello World');
  });

  it('should return empty string when no text set', async () => {
    const node = new TextInputNode();
    
    const result = await node.execute({});
    
    expect(result.text).toBe('');
  });
});

describe('NumberInputNode', () => {
  it('should create with default value of 0', () => {
    const node = new NumberInputNode();
    expect(node.label).toBe('Number Input');
    expect(node.getControlValue('number')).toBe(0);
  });

  it('should execute and return number', async () => {
    const node = new NumberInputNode();
    node.setControlValue('number', 42);
    
    const result = await node.execute({});
    
    expect(result.number).toBe(42);
  });
});

describe('CustomJSNode', () => {
  it('should create with default code', () => {
    const node = new CustomJSNode();
    expect(node.label).toBe('Custom JS');
    expect(node.category).toBe('Processing');
  });

  it('should execute custom code', async () => {
    const node = new CustomJSNode();
    node.setCode(`
      return {
        output: input1 + ' ' + input2
      };
    `);
    
    const result = await node.execute({ input1: 'Hello', input2: 42 });
    
    expect(result.output).toBe('Hello 42');
  });

  it('should handle code errors gracefully', async () => {
    const node = new CustomJSNode();
    node.setCode('throw new Error("Test error");');
    
    const result = await node.execute({});
    
    expect(result.output).toBeNull();
    expect(result.error).toContain('Test error');
  });

  it('should use default values for missing inputs', async () => {
    const node = new CustomJSNode();
    node.setCode('return { output: input1 + " " + input2 };');
    
    const result = await node.execute({});
    
    expect(result.output).toBe(' 0');
  });
});
