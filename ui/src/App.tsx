import { Routes, Route } from "react-router-dom";
import Home from "./pages/Home.tsx";
import Setup from "./pages/Setup.tsx";
import TtsPage from "./pages/tts/index.tsx";
import AudioConversionPage from "./pages/audio-conversion/index.tsx";
import MusicPage from "./pages/music/index.tsx";
import ToolsPage from "./pages/tools/index.tsx";
import OutputsPage from "./pages/outputs/index.tsx";
import ExtensionsPage from "./pages/extensions/index.tsx";
import SettingsPage from "./pages/settings/index.tsx";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/setup" element={<Setup />} />
      <Route path="/tts" element={<TtsPage />} />
      <Route path="/audio-conversion" element={<AudioConversionPage />} />
      <Route path="/music" element={<MusicPage />} />
      <Route path="/tools" element={<ToolsPage />} />
      <Route path="/outputs" element={<OutputsPage />} />
      <Route path="/extensions" element={<ExtensionsPage />} />
      <Route path="/settings" element={<SettingsPage />} />
    </Routes>
  );
}

