import { proxyGradioFilesInResult } from './proxyGradioFile';
import { defaultBackend } from './defaultBackend';

describe('proxyGradioFilesInResult', () => {
  test('processes Gradio API response with file URLs', () => {
    // Example input from the user's request
    const input = {
      data: [
        {
          "path": "C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav",
          "url": `${defaultBackend}gradio_api/file=C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav`,
          "size": null,
          "orig_name": "audio.wav",
          "mime_type": null,
          "is_stream": false,
          "meta": {
            "_type": "gradio.FileData"
          }
        },
        {
          "_version": "0.0.1",
          "_hash_version": "0.0.2",
          "text": "test",
          "language": "eng",
          "speaking_rate": 1,
          "noise_scale": 0.667,
          "noise_scale_duration": 0.8,
          "seed": "87903817",
          "outputs": null,
          "date": "2025-04-23 16:53:23.107821",
          "hash": "791c71ce8b0003ac376bd37095276ac8cf9212e859b6a777f8afc62752f11c23"
        },
        "outputs\\2025-04-23_16-53-23__mms__test"
      ]
    };

    // Expected output after processing
    const expected = {
      data: [
        {
          "path": "C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav",
          "url": `/api/gradio-file-proxy/gradio_api/file=C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav`,
          "size": null,
          "orig_name": "audio.wav",
          "mime_type": null,
          "is_stream": false,
          "meta": {
            "_type": "gradio.FileData"
          }
        },
        {
          "_version": "0.0.1",
          "_hash_version": "0.0.2",
          "text": "test",
          "language": "eng",
          "speaking_rate": 1,
          "noise_scale": 0.667,
          "noise_scale_duration": 0.8,
          "seed": "87903817",
          "outputs": null,
          "date": "2025-04-23 16:53:23.107821",
          "hash": "791c71ce8b0003ac376bd37095276ac8cf9212e859b6a777f8afc62752f11c23"
        },
        "outputs\\2025-04-23_16-53-23__mms__test"
      ]
    };

    // Process the response with proxyGradioFilesInResult
    const result = proxyGradioFilesInResult(input);
    
    // Test the entire processed response
    expect(result).toEqual(expected);
    
    // Test that the URL was transformed correctly
    expect((result.data[0] as any).url).toBe((expected.data[0] as any).url);
    
    // Test that non-URL objects remain unchanged
    expect(result.data[1]).toBe(input.data[1]);
    expect(result.data[2]).toBe(input.data[2]);
  });

  test('handles non-array data property', () => {
    // Test with a non-array data property
    const input = {
      data: "not an array"
    };

    // The function should return the input unchanged
    const result = proxyGradioFilesInResult(input);
    expect(result).toEqual(input);
    expect(result.data).toEqual(input.data);
  });

  test('preserves other properties in the response object', () => {
    // Test with additional properties in the response object
    const input = {
      data: [
        {
          "url": `${defaultBackend}file/audio.wav`,
          "orig_name": "audio.wav"
        }
      ],
      status: "success",
      duration: 1.234
    };

    // Expected output after processing
    const expected = {
      data: [
        {
          "url": "/api/gradio-file-proxy/file/audio.wav",
          "orig_name": "audio.wav"
        }
      ],
      status: "success",
      duration: 1.234
    };

    // Process the response with proxyGradioFilesInResult
    const result = proxyGradioFilesInResult(input);
    
    // Test the entire processed response
    expect(result).toEqual(expected);
    
    // Test that additional properties are preserved
    expect(result.status).toBe(input.status);
    expect(result.duration).toBe(input.duration);
  });

  test('handles empty data array', () => {
    // Test with an empty data array
    const input = {
      data: []
    };

    // The function should return the input unchanged
    const result = proxyGradioFilesInResult(input);
    expect(result).toEqual(input);
    expect(result.data).toEqual(input.data);
  });

  test('simulates usage in a Gradio API client', () => {
    // Simulate a Gradio API client response
    const mockGradioResponse = {
      data: [
        {
          "path": "C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav",
          "url": `${defaultBackend}gradio_api/file=C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav`,
          "size": null,
          "orig_name": "audio.wav"
        },
        {
          "metadata": {
            "model": "mms",
            "text": "test"
          }
        }
      ]
    };

    // Simulate the Gradio API client's predict function
    const mockPredict = () => Promise.resolve(mockGradioResponse);
    
    // Simulate the chained then call with proxyGradioFilesInResult
    const gradioPredict = () => mockPredict().then(proxyGradioFilesInResult);
    
    // Test the function
    return gradioPredict().then(result => {
      // Test that the URL was transformed correctly
      expect((result.data[0] as any).url).toBe(`/api/gradio-file-proxy/gradio_api/file=C:\\Users\\rob\\AppData\\Local\\Temp\\gradio\\30e19519e4b162ce416e223eafc29da1dfa552ba75a59a831e68da763f5202a8\\audio.wav`);
      
      // Test that non-URL objects remain unchanged
      expect(result.data[1]).toBe(mockGradioResponse.data[1]);
    });
  });
});
