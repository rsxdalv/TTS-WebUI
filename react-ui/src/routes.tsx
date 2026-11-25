import React, { Suspense, lazy } from "react";
import { Route, Routes } from "react-router-dom";

/**
 * The routes of the app.
 *
 * The page components are in src/pages; this table lists them with their
 * paths.
 *
 * The API routes are not here: they belong to the Go server. The Vite dev
 * server proxies /api to it.
 */

/** The home page: the model list. */
const Home = lazy(() => import("./pages/index"));

/** The model lists. */
const TextToSpeech = lazy(() => import("./pages/text-to-speech"));
const AudioMusicGeneration = lazy(() =>
  import("./pages/audio-music-generation")
);
const AudioConversion = lazy(() => import("./pages/audio-conversion"));

/** The pipeline page. */
const Pipeline = lazy(() => import("./pages/pipeline"));

/** The tools pages. */
const Tools = lazy(() => import("./pages/tools"));
const ToolsGpuInfo = lazy(() => import("./pages/tools/gpu_info"));
const ToolsApiConfig = lazy(() => import("./pages/tools/api_config"));
const ToolsPipList = lazy(() => import("./pages/tools/pip_list"));
const ToolsExtensions = lazy(() => import("./pages/tools/extensions"));
const ToolsRemixer = lazy(() => import("./pages/tools/simple_remixer"));
const ToolsYouTube = lazy(() => import("./pages/tools/youtube_tutorials"));
const OpenAIEndpoints = lazy(() => import("./pages/openai"));
const EnginesIndex = lazy(() => import("./pages/engines"));
const EnginePage = lazy(() => import("./pages/engines/engine"));
const ToolsLogViewer = lazy(() => import("./pages/tools/log_viewer"));
const ToolsModelLocation = lazy(() => import("./pages/tools/model_location"));
const ToolsServerSettings = lazy(() => import("./pages/tools/server_settings"));

/** The outputs pages. */
const Outputs = lazy(() => import("./pages/outputs"));
const OutputsFfmpegMetadata = lazy(() =>
  import("./pages/outputs/ffmpeg-metadata-page")
);
const OutputsGenerations = lazy(() => import("./pages/outputs/generations"));
const OutputsVoiceDrafts = lazy(() => import("./pages/outputs/voice-drafts"));

/** The text-to-speech pages. */
const Bark = lazy(() => import("./pages/text-to-speech/bark"));
const BarkGeneration = lazy(() =>
  import("./pages/text-to-speech/bark/generation")
);
const BarkVoiceGeneration = lazy(() =>
  import("./pages/text-to-speech/bark/bark_voice_generation")
);
const BarkVoices = lazy(() => import("./pages/text-to-speech/bark/voices"));
const BarkSettings = lazy(() =>
  import("./pages/text-to-speech/bark/bark_settings")
);
const BarkVocosNpz = lazy(() =>
  import("./pages/text-to-speech/bark/vocos_npz")
);
const Chatterbox = lazy(() => import("./pages/text-to-speech/chatterbox"));
const Kokoro = lazy(() => import("./pages/text-to-speech/kokoro"));
const MahaTts = lazy(() => import("./pages/text-to-speech/maha-tts"));
const MinimaxTts = lazy(() => import("./pages/text-to-speech/minimax-tts"));
const Mms = lazy(() => import("./pages/text-to-speech/mms"));
const Tortoise = lazy(() => import("./pages/text-to-speech/tortoise"));
const Vallex = lazy(() => import("./pages/text-to-speech/vallex"));

/** The audio-conversion pages. */
const Demucs = lazy(() => import("./pages/audio-conversion/demucs"));
const Rvc = lazy(() => import("./pages/audio-conversion/rvc"));
const VocosWav = lazy(() => import("./pages/audio-conversion/vocos_wav"));

/** The audio-music-generation pages. */
const AceStep = lazy(() => import("./pages/audio-music-generation/ace-step"));
const Magnet = lazy(() => import("./pages/audio-music-generation/magnet"));
const Musicgen = lazy(() => import("./pages/audio-music-generation/musicgen"));
const StableAudio = lazy(() =>
  import("./pages/audio-music-generation/stable-audio")
);

/** The extension pages. */
const Extension = lazy(() => import("./pages/extensions/[name]"));
const HuggingFaceCacheManager = lazy(() =>
  import("./pages/extensions/huggingface_cache_manager")
);

/** The remaining pages. */
const Placeholder = lazy(() => import("./pages/placeholder"));
const TestGradioProxy = lazy(() => import("./pages/test-gradio-proxy"));

/**
 * The outputs page takes the collection name as a parameter. The data comes
 * from the API in the browser (see src/data/useGoData.ts), so the props are
 * empty.
 */
const OutputsByName = lazy(() => import("./pages/outputs/[name]"));

/** The loading state of a page, while a page loads. */
const Loading = () => (
  <div className="flex min-h-screen w-full items-center justify-start">
    <div className="h-8 w-8 animate-spin rounded-full border-2 border-gray-300 border-t-gray-900" />
  </div>
);

export const AppRoutes = () => (
  <Suspense fallback={<Loading />}>
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/pipeline" element={<Pipeline />} />
      <Route path="/placeholder" element={<Placeholder />} />
      <Route path="/test-gradio-proxy" element={<TestGradioProxy />} />

      <Route path="/text-to-speech" element={<TextToSpeech />} />
      <Route path="/text-to-speech/bark" element={<Bark />} />
      <Route
        path="/text-to-speech/bark/generation"
        element={<BarkGeneration />}
      />
      <Route
        path="/text-to-speech/bark/bark_voice_generation"
        element={<BarkVoiceGeneration />}
      />
      <Route path="/text-to-speech/bark/voices" element={<BarkVoices />} />
      <Route
        path="/text-to-speech/bark/bark_settings"
        element={<BarkSettings />}
      />
      <Route path="/text-to-speech/bark/vocos_npz" element={<BarkVocosNpz />} />
      <Route path="/text-to-speech/chatterbox" element={<Chatterbox />} />
      <Route path="/text-to-speech/kokoro" element={<Kokoro />} />
      <Route path="/text-to-speech/maha-tts" element={<MahaTts />} />
      <Route path="/text-to-speech/minimax-tts" element={<MinimaxTts />} />
      <Route path="/text-to-speech/mms" element={<Mms />} />
      <Route path="/text-to-speech/tortoise" element={<Tortoise />} />
      <Route path="/text-to-speech/vallex" element={<Vallex />} />

      <Route path="/audio-music-generation" element={<AudioMusicGeneration />} />
      <Route path="/audio-music-generation/ace-step" element={<AceStep />} />
      <Route path="/audio-music-generation/magnet" element={<Magnet />} />
      <Route path="/audio-music-generation/musicgen" element={<Musicgen />} />
      <Route
        path="/audio-music-generation/stable-audio"
        element={<StableAudio />}
      />

      <Route path="/audio-conversion" element={<AudioConversion />} />
      <Route path="/audio-conversion/demucs" element={<Demucs />} />
      <Route path="/audio-conversion/rvc" element={<Rvc />} />
      <Route path="/audio-conversion/vocos_wav" element={<VocosWav />} />

      <Route path="/outputs" element={<Outputs />} />
      <Route
        path="/outputs/ffmpeg-metadata-page"
        element={<OutputsFfmpegMetadata />}
      />
      <Route path="/outputs/generations" element={<OutputsGenerations />} />
      <Route path="/outputs/voice-drafts" element={<OutputsVoiceDrafts />} />

      <Route path="/tools" element={<Tools />} />
      <Route path="/tools/gpu_info" element={<ToolsGpuInfo />} />
      <Route path="/tools/api_config" element={<ToolsApiConfig />} />
      <Route path="/tools/pip_list" element={<ToolsPipList />} />
      <Route path="/openai" element={<OpenAIEndpoints />} />
      <Route path="/engines" element={<EnginesIndex />} />
      <Route path="/engine" element={<EnginePage />} />
      <Route path="/tools/extensions" element={<ToolsExtensions />} />
      <Route path="/tools/simple_remixer" element={<ToolsRemixer />} />
      <Route path="/tools/youtube_tutorials" element={<ToolsYouTube />} />
      <Route path="/tools/log_viewer" element={<ToolsLogViewer />} />
      <Route path="/tools/model_location" element={<ToolsModelLocation />} />
      <Route path="/tools/server_settings" element={<ToolsServerSettings />} />

      <Route
        path="/extensions/huggingface_cache_manager"
        element={<HuggingFaceCacheManager />}
      />
      {/* The extension page comes after the fixed extension page, so
          /extensions/huggingface_cache_manager wins. */}
      <Route path="/extensions/:name" element={<Extension />} />

      {/* The outputs page takes the collection name as a parameter. */}
      <Route path="/outputs/:name" element={<OutputsByName />} />
    </Routes>
  </Suspense>
);
