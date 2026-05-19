import {
  audioSocket,
  textSocket,
  numberSocket,
  anySocket,
  AudioSocket,
  TextSocket,
  NumberSocket,
  AnySocket,
} from '../types';

describe('Socket Types', () => {
  describe('AudioSocket', () => {
    it('should have correct name', () => {
      expect(audioSocket.name).toBe('Audio');
    });

    it('should be singleton instance', () => {
      const socket1 = audioSocket;
      const socket2 = audioSocket;
      expect(socket1).toBe(socket2);
    });
  });

  describe('TextSocket', () => {
    it('should have correct name', () => {
      expect(textSocket.name).toBe('Text');
    });

    it('should be singleton instance', () => {
      const socket1 = textSocket;
      const socket2 = textSocket;
      expect(socket1).toBe(socket2);
    });
  });

  describe('NumberSocket', () => {
    it('should have correct name', () => {
      expect(numberSocket.name).toBe('Number');
    });

    it('should be singleton instance', () => {
      const socket1 = numberSocket;
      const socket2 = numberSocket;
      expect(socket1).toBe(socket2);
    });
  });

  describe('AnySocket', () => {
    it('should have correct name', () => {
      expect(anySocket.name).toBe('Any');
    });

    it('should be singleton instance', () => {
      const socket1 = anySocket;
      const socket2 = anySocket;
      expect(socket1).toBe(socket2);
    });
  });

  describe('Socket classes', () => {
    it('should create new instances with correct names', () => {
      const audio = new AudioSocket();
      const text = new TextSocket();
      const number = new NumberSocket();
      const any = new AnySocket();

      expect(audio.name).toBe('Audio');
      expect(text.name).toBe('Text');
      expect(number.name).toBe('Number');
      expect(any.name).toBe('Any');
    });
  });
});
