import type { NextApiRequest, NextApiResponse } from 'next';

const GRADIO_BACKEND = process.env.GRADIO_BACKEND || 'http://localhost:7860';

export default async function handler(
  req: NextApiRequest,
  res: NextApiResponse
) {
  const { endpoint } = req.query;
  
  if (!endpoint || typeof endpoint !== 'string') {
    return res.status(400).json({ error: 'Missing endpoint parameter' });
  }
  
  try {
    const url = `${GRADIO_BACKEND}/api/${endpoint}`;
    
    const response = await fetch(url, {
      method: req.method,
      headers: {
        'Content-Type': 'application/json',
        ...(process.env.GRADIO_AUTH ? {
          'Authorization': `Basic ${Buffer.from(process.env.GRADIO_AUTH).toString('base64')}`
        } : {}),
      },
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
      error: error instanceof Error ? error.message : 'Internal server error' 
    });
  }
}
