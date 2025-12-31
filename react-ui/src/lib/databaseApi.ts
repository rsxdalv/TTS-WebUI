/**
 * TTS WebUI Database API Client
 * 
 * Client for the REST API that provides database access for:
 * - Generation history
 * - Favorites
 * - Voice profiles
 * - User preferences
 */

const API_BASE = process.env.NEXT_PUBLIC_DB_API_URL || 'http://127.0.0.1:7774/api';

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  body?: any;
  headers?: Record<string, string>;
}

class DatabaseApiClient {
  private apiKey: string | null = null;

  setApiKey(key: string) {
    this.apiKey = key;
    if (typeof window !== 'undefined') {
      localStorage.setItem('tts_webui_api_key', key);
    }
  }

  getApiKey(): string | null {
    if (this.apiKey) return this.apiKey;
    if (typeof window !== 'undefined') {
      return localStorage.getItem('tts_webui_api_key');
    }
    return null;
  }

  private async request<T>(endpoint: string, options: RequestOptions = {}): Promise<T> {
    const { method = 'GET', body, headers = {} } = options;
    
    const apiKey = this.getApiKey();
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`;
    }
    
    const config: RequestInit = {
      method,
      headers: {
        'Content-Type': 'application/json',
        ...headers,
      },
    };
    
    if (body) {
      config.body = JSON.stringify(body);
    }
    
    const response = await fetch(`${API_BASE}${endpoint}`, config);
    
    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(error.error || 'API request failed');
    }
    
    return response.json();
  }

  // ============================================================================
  // Health & Status
  // ============================================================================

  async health(): Promise<{ status: string; database: string; version: string }> {
    return this.request('/health');
  }

  // ============================================================================
  // API Keys
  // ============================================================================

  async createApiKey(name?: string): Promise<{ key: string; prefix: string; message: string }> {
    return this.request('/keys', { method: 'POST', body: { name } });
  }

  async listApiKeys(): Promise<{ keys: ApiKeyInfo[] }> {
    return this.request('/keys');
  }

  async revokeApiKey(keyId: number): Promise<void> {
    return this.request(`/keys/${keyId}`, { method: 'DELETE' });
  }

  // ============================================================================
  // Generations
  // ============================================================================

  async listGenerations(params?: GenerationListParams): Promise<GenerationListResponse> {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());
    if (params?.model_type) searchParams.set('model_type', params.model_type);
    if (params?.model_name) searchParams.set('model_name', params.model_name);
    if (params?.status) searchParams.set('status', params.status);
    
    const query = searchParams.toString();
    return this.request(`/generations${query ? `?${query}` : ''}`);
  }

  async getGeneration(id: number): Promise<Generation> {
    return this.request(`/generations/${id}`);
  }

  async createGeneration(data: CreateGenerationData): Promise<{ id: number }> {
    return this.request('/generations', { method: 'POST', body: data });
  }

  async updateGeneration(id: number, data: Partial<Generation>): Promise<void> {
    return this.request(`/generations/${id}`, { method: 'PATCH', body: data });
  }

  async deleteGeneration(id: number): Promise<void> {
    return this.request(`/generations/${id}`, { method: 'DELETE' });
  }

  // ============================================================================
  // Favorites
  // ============================================================================

  async listFavorites(params?: { limit?: number; offset?: number }): Promise<{ favorites: Favorite[] }> {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());
    
    const query = searchParams.toString();
    return this.request(`/favorites${query ? `?${query}` : ''}`);
  }

  async addFavorite(data: { generation_id: number; name?: string; notes?: string; tags?: string[] }): Promise<{ id: number }> {
    return this.request('/favorites', { method: 'POST', body: data });
  }

  async removeFavorite(favoriteId: number): Promise<void> {
    return this.request(`/favorites/${favoriteId}`, { method: 'DELETE' });
  }

  async unfavoriteGeneration(generationId: number): Promise<void> {
    return this.request(`/favorites/generation/${generationId}`, { method: 'DELETE' });
  }

  async isFavorited(generationId: number): Promise<{ is_favorited: boolean }> {
    return this.request(`/favorites/check/${generationId}`);
  }

  // ============================================================================
  // Voice Profiles
  // ============================================================================

  async listVoiceProfiles(modelType?: string): Promise<{ profiles: VoiceProfile[] }> {
    const query = modelType ? `?model_type=${modelType}` : '';
    return this.request(`/voice-profiles${query}`);
  }

  async getVoiceProfile(id: number): Promise<VoiceProfile> {
    return this.request(`/voice-profiles/${id}`);
  }

  async createVoiceProfile(data: CreateVoiceProfileData): Promise<{ id: number }> {
    return this.request('/voice-profiles', { method: 'POST', body: data });
  }

  async updateVoiceProfile(id: number, data: Partial<VoiceProfile>): Promise<void> {
    return this.request(`/voice-profiles/${id}`, { method: 'PATCH', body: data });
  }

  async deleteVoiceProfile(id: number): Promise<void> {
    return this.request(`/voice-profiles/${id}`, { method: 'DELETE' });
  }

  // ============================================================================
  // User Preferences
  // ============================================================================

  async getAllPreferences(): Promise<{ preferences: Record<string, Record<string, any>> }> {
    return this.request('/preferences');
  }

  async getCategoryPreferences(category: string): Promise<{ preferences: Record<string, any> }> {
    return this.request(`/preferences?category=${category}`);
  }

  async getPreference(category: string, key: string): Promise<{ value: any }> {
    return this.request(`/preferences/${category}/${key}`);
  }

  async setPreference(category: string, key: string, value: any): Promise<void> {
    return this.request(`/preferences/${category}/${key}`, { method: 'PUT', body: { value } });
  }

  async deletePreference(category: string, key: string): Promise<void> {
    return this.request(`/preferences/${category}/${key}`, { method: 'DELETE' });
  }

  async setBulkPreferences(preferences: Record<string, Record<string, any>>): Promise<void> {
    return this.request('/preferences/bulk', { method: 'PUT', body: { preferences } });
  }

  // ============================================================================
  // Rescan & Statistics
  // ============================================================================

  async rescan(): Promise<RescanResult> {
    return this.request('/rescan', { method: 'POST' });
  }

  async getStats(): Promise<DatabaseStats> {
    return this.request('/stats');
  }
}

// ============================================================================
// Types
// ============================================================================

export interface ApiKeyInfo {
  id: number;
  key_prefix: string;
  name: string;
  is_active: boolean;
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
}

export interface Generation {
  id: number;
  user_id: number;
  filename: string;
  filepath: string;
  file_exists: boolean;
  file_size: number | null;
  duration_seconds: number | null;
  model_name: string | null;
  model_type: string | null;
  text: string | null;
  language: string | null;
  voice: string | null;
  parameters: Record<string, any>;
  generation_time_seconds: number | null;
  created_at: string;
  status: string;
  error_message: string | null;
}

export interface GenerationListParams {
  limit?: number;
  offset?: number;
  model_type?: string;
  model_name?: string;
  status?: string;
}

export interface GenerationListResponse {
  generations: Generation[];
  total: number;
  limit: number;
  offset: number;
}

export interface CreateGenerationData {
  filename: string;
  filepath: string;
  model_name?: string;
  model_type?: string;
  text?: string;
  language?: string;
  voice?: string;
  parameters?: Record<string, any>;
  generation_time_seconds?: number;
  file_size?: number;
  duration_seconds?: number;
  status?: string;
  error_message?: string;
}

export interface Favorite {
  favorite_id: number;
  favorite_name: string | null;
  notes: string | null;
  tags: string[];
  favorited_at: string;
  // Plus all Generation fields
  id: number;
  filename: string;
  filepath: string;
  model_name: string | null;
  text: string | null;
  created_at: string;
}

export interface VoiceProfile {
  id: number;
  user_id: number;
  name: string;
  description: string | null;
  model_type: string;
  config: Record<string, any>;
  reference_audio_path: string | null;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateVoiceProfileData {
  name: string;
  model_type: string;
  config: Record<string, any>;
  description?: string;
  reference_audio_path?: string;
  is_default?: boolean;
}

export interface RescanResult {
  scanned: number;
  added: number;
  marked_missing: number;
  already_tracked: number;
  errors: string[];
  directories_scanned: string[];
}

export interface DatabaseStats {
  generations: {
    total: number;
    by_model: { model_type: string; count: number }[];
  };
  favorites: {
    total: number;
  };
  voice_profiles: {
    total: number;
  };
}

// Export singleton instance
export const dbApi = new DatabaseApiClient();
export default dbApi;
