import { ClassicPreset } from 'rete';
import { audioSocket, textSocket, numberSocket } from '../types';

export abstract class BaseNode extends ClassicPreset.Node {
  width = 200;
  height = 180;
  
  // Node category for context menu organization
  abstract readonly category: string;
  
  // Store control values
  protected controlValues: Record<string, any> = {};
  
  constructor(label: string) {
    super(label);
  }
  
  // Abstract method for node execution
  abstract execute(inputs: Record<string, any>): Promise<Record<string, any>>;
  
  // Helper to get socket by type name
  protected getSocket(type: 'audio' | 'text' | 'number'): ClassicPreset.Socket {
    switch (type) {
      case 'audio':
        return audioSocket;
      case 'text':
        return textSocket;
      case 'number':
        return numberSocket;
      default:
        return textSocket;
    }
  }
  
  // Get control value
  getControlValue<T>(key: string): T | undefined {
    const control = this.controls[key];
    if (control instanceof ClassicPreset.InputControl) {
      return control.value as T;
    }
    return this.controlValues[key];
  }
  
  // Set control value
  setControlValue(key: string, value: any): void {
    const control = this.controls[key];
    if (control instanceof ClassicPreset.InputControl) {
      control.setValue(value);
    }
    this.controlValues[key] = value;
  }
  
  // Serialize the node state
  serialize(): {
    id: string;
    type: string;
    label: string;
    controls: Record<string, any>;
  } {
    const controls: Record<string, any> = {};
    for (const [key, control] of Object.entries(this.controls)) {
      if (control instanceof ClassicPreset.InputControl) {
        controls[key] = control.value;
      }
    }
    return {
      id: this.id,
      type: this.constructor.name,
      label: this.label,
      controls: { ...controls, ...this.controlValues },
    };
  }
  
  // Deserialize control values
  deserialize(data: { controls: Record<string, any> }): void {
    for (const [key, value] of Object.entries(data.controls)) {
      this.setControlValue(key, value);
    }
  }
}
