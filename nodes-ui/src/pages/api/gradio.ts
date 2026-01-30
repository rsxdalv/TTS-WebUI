import type { NextApiRequest, NextApiResponse } from 'next';

const GRADIO_BACKEND = process.env.GRADIO_BACKEND || 'http://localhost:7860';

// Validate endpoint to prevent path traversal
function isValidEndpoint(endpoint: string): boolean {
  // Only allow alphanumeric, hyphens, underscores, and forward slashes (for namespaced endpoints)
  return /^[a-zA-Z0-9_\-\/]+$/.test(endpoint) && !endpoint.includes('..');
}

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse
) {
  const { endpoint } = req.query;
  
  if (!endpoint || typeof endpoint !== 'string') {
    return res.status(400).json({ error: 'Missing endpoint parameter' });
  }
  
  if (!isValidEndpoint(endpoint)) {
    return res.status(400).json({ error: 'Invalid endpoint format' });
  }
  
  try {
    const url = `${GRADIO_BACKEND}/api/${encodeURIComponent(endpoint).replace(/%2F/g, '/')}`;
    
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    
    // GRADIO_AUTH should be in username:password format
    if (process.env.GRADIO_AUTH) {
      headers['Authorization'] = `Basic ${Buffer.from(process.env.GRADIO_AUTH).toString('base64')}`;
    }
    
    const response = await fetch(url, {
      method: req.method,
      headers,
      body: req.method !== 'GET' ? JSON.stringify(req.body) : undefined,
    });
    
    if (!response.ok) {
      return res.status(response.status).json({ 
        error: `Gradio API error: ${response.statusText}` 
      });
    }
    
    const data = await response.json();
    return res.status(200).json(data);
  } catch (error) {
    console.error('Gradio proxy error:', error);
    return res.status(500).json({ 
      error: 'Internal server error' 
    });
  }
}
