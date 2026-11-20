// Example custom node definitions
// These can be loaded dynamically to extend the node editor

export const customNodes = [
  {
    id: 'text-concat',
    name: 'Text Concatenate',
    category: 'Text Processing',
    description: 'Concatenates two text inputs with an optional separator',
    inputs: [
      { key: 'text1', label: 'Text 1', socket: 'text' },
      { key: 'text2', label: 'Text 2', socket: 'text' },
    ],
    outputs: [
      { key: 'result', label: 'Result', socket: 'text' },
    ],
    code: `
      const separator = ' ';
      return {
        result: (text1 || '') + separator + (text2 || '')
      };
    `,
  },
  {
    id: 'audio-volume',
    name: 'Audio Volume',
    category: 'Audio Processing',
    description: 'Adjusts the volume of an audio input (placeholder)',
    inputs: [
      { key: 'audio', label: 'Audio', socket: 'audio' },
      { key: 'volume', label: 'Volume', socket: 'number' },
    ],
    outputs: [
      { key: 'audio', label: 'Audio', socket: 'audio' },
    ],
    code: `
      // In a real implementation, this would modify the audio
      // For now, just pass through the audio
      console.log('Volume adjustment:', volume);
      return { audio };
    `,
  },
  {
    id: 'number-math',
    name: 'Number Math',
    category: 'Math',
    description: 'Performs basic math operations on two numbers',
    inputs: [
      { key: 'a', label: 'A', socket: 'number' },
      { key: 'b', label: 'B', socket: 'number' },
    ],
    outputs: [
      { key: 'sum', label: 'Sum', socket: 'number' },
      { key: 'product', label: 'Product', socket: 'number' },
    ],
    code: `
      const numA = Number(a) || 0;
      const numB = Number(b) || 0;
      return {
        sum: numA + numB,
        product: numA * numB
      };
    `,
  },
];

export default customNodes;
