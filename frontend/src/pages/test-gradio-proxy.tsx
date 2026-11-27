import React, { useState } from 'react';

export default function TestGradioProxy() {
  const [fileUrl, setFileUrl] = useState('');
  const [proxyUrl, setProxyUrl] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!fileUrl) {
      setError('Please enter a file URL');
      return;
    }
    
    setIsLoading(true);
    setError('');
    
    try {
      // Handle gradio_api/file= format URLs
      if (fileUrl.includes('/gradio_api/file=')) {
        const gradioApiFilePrefix = "http://127.0.0.1:7770/gradio_api/file=";
        if (fileUrl.startsWith(gradioApiFilePrefix)) {
          const filePath = fileUrl.substring(gradioApiFilePrefix.length);
          setProxyUrl(`/gradio-file-proxy/gradio_api/file=${filePath}`);
        } else {
          // Extract the file path part
          const parts = fileUrl.split('/gradio_api/file=');
          if (parts.length > 1) {
            setProxyUrl(`/gradio-file-proxy/gradio_api/file=${parts[1]}`);
          }
        }
      } 
      // Handle standard backend URLs
      else if (fileUrl.startsWith('http://127.0.0.1:7770/')) {
        setProxyUrl(fileUrl.replace('http://127.0.0.1:7770/', '/gradio-file-proxy/'));
      } 
      // If it's already a proxy URL, use it directly
      else if (fileUrl.startsWith('/gradio-file-proxy/')) {
        setProxyUrl(fileUrl);
      }
      // Unknown format
      else {
        setError('Unsupported URL format');
      }
    } catch (err) {
      setError('Error processing URL');
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div style={{ maxWidth: '800px', margin: '0 auto', padding: '20px' }}>
      <h1>Test Gradio File Proxy</h1>
      
      <form onSubmit={handleSubmit} style={{ marginBottom: '20px' }}>
        <div style={{ marginBottom: '10px' }}>
          <label htmlFor="fileUrl" style={{ display: 'block', marginBottom: '5px' }}>
            Enter Gradio File URL:
          </label>
          <input
            type="text"
            id="fileUrl"
            value={fileUrl}
            onChange={(e) => setFileUrl(e.target.value)}
            placeholder="http://127.0.0.1:7770/gradio_api/file=C:/Users/..."
            style={{ width: '100%', padding: '8px' }}
          />
        </div>
        
        <button 
          type="submit" 
          disabled={isLoading}
          style={{
            padding: '8px 16px',
            backgroundColor: '#0070f3',
            color: 'white',
            border: 'none',
            borderRadius: '4px',
            cursor: isLoading ? 'not-allowed' : 'pointer'
          }}
        >
          {isLoading ? 'Processing...' : 'Generate Proxy URL'}
        </button>
      </form>
      
      {error && (
        <div style={{ color: 'red', marginBottom: '20px' }}>
          {error}
        </div>
      )}
      
      {proxyUrl && (
        <div>
          <h2>Proxy URL:</h2>
          <div style={{ 
            padding: '10px', 
            backgroundColor: '#f0f0f0', 
            borderRadius: '4px',
            wordBreak: 'break-all',
            marginBottom: '20px'
          }}>
            {proxyUrl}
          </div>
          
          <h2>Test Result:</h2>
          {proxyUrl.endsWith('.wav') || proxyUrl.endsWith('.mp3') ? (
            <div>
              <h3>Audio:</h3>
              <audio controls src={proxyUrl} style={{ width: '100%' }}>
                Your browser does not support the audio element.
              </audio>
            </div>
          ) : proxyUrl.endsWith('.jpg') || proxyUrl.endsWith('.jpeg') || proxyUrl.endsWith('.png') ? (
            <div>
              <h3>Image:</h3>
              <img src={proxyUrl} alt="Proxied file" style={{ maxWidth: '100%' }} />
            </div>
          ) : (
            <div>
              <p>File type not recognized for preview. <a href={proxyUrl} target="_blank" rel="noopener noreferrer">Click here to open the file</a></p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
