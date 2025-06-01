"use client";

import { useState, useEffect } from "react";
import {
  Clipboard,
  Plus,
  Trash2,
  Save,
  RefreshCw,
  Play,
  Copy,
  Check,
  Minus,
  Edit,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/hooks/use-toast";
import { Slider } from "@/components/ui/slider";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

// Predefined voice options (these would come from an API in production)
const VOICE_OPTIONS = {
  "🇺🇸 🚺 Heart ❤️": "af_heart",
  "🇺🇸 🚺 Bella 🔥": "af_bella",
  "🇺🇸 🚺 Nicole 🎧": "af_nicole",
  "🇺🇸 🚺 Aoede": "af_aoede",
  "🇺🇸 🚺 Kore": "af_kore",
  "🇺🇸 🚺 Sarah": "af_sarah",
  "🇺🇸 🚺 Nova": "af_nova",
  "🇺🇸 🚺 Sky": "af_sky",
  "🇺🇸 🚺 Alloy": "af_alloy",
  "🇺🇸 🚺 Jessica": "af_jessica",
  "🇺🇸 🚺 River": "af_river",
  "🇺🇸 🚹 Michael": "am_michael",
  "🇺🇸 🚹 Fenrir": "am_fenrir",
  "🇺🇸 🚹 Puck": "am_puck",
  "🇺🇸 🚹 Echo": "am_echo",
  "🇺🇸 🚹 Eric": "am_eric",
  "🇺🇸 🚹 Liam": "am_liam",
  "🇺🇸 🚹 Onyx": "am_onyx",
  "🇺🇸 🚹 Santa": "am_santa",
  "🇺🇸 🚹 Adam": "am_adam",
  "🇬🇧 🚺 Emma": "bf_emma",
  "🇬🇧 🚺 Isabella": "bf_isabella",
  "🇬🇧 🚺 Alice": "bf_alice",
  "🇬🇧 🚺 Lily": "bf_lily",
  "🇬🇧 🚹 George": "bm_george",
  "🇬🇧 🚹 Fable": "bm_fable",
  // # "🇬🇧 🚹 Lewis": "bm_lewis", # Suspicious data.
  "🇬🇧 🚹 Daniel": "bm_daniel",
  "🇪🇸 🚺 Dora": "ef_dora",
  "🇪🇸 🚹 Alex": "em_alex",
  "🇪🇸 🚹 Santa": "em_santa",
  "🇫🇷 🚺 Siwis": "ff_siwis",
  "🇮🇳 🚺 Alpha": "hf_alpha",
  "🇮🇳 🚺 Beta": "hf_beta",
  "🇮🇳 🚹 Omega": "hm_omega",
  "🇮🇳 🚹 Psi": "hm_psi",
  "🇮🇹 🚺 Sara": "if_sara",
  "🇮🇹 🚹 Nicola": "im_nicola",
  "🇧🇷 🚺 Dora": "pf_dora",
  "🇧🇷 🚹 Alex": "pm_alex",
  "🇧🇷 🚹 Santa": "pm_santa",
  // # requires fugashi
  "🇯🇵 🚺 Alpha": "jf_alpha",
  "🇯🇵 🚺 Gongitsune": "jf_gongitsune",
  "🇯🇵 🚺 Nezumi": "jf_nezumi",
  "🇯🇵 🚺 Tebukuro": "jf_tebukuro",
  "🇯🇵 🚹 Kumo": "jm_kumo",
  // # requires misaki
  "🇨🇳 🚺 Xiaobei": "zf_xiaobei",
  "🇨🇳 🚺 Xiaoni": "zf_xiaoni",
  "🇨🇳 🚺 Xiaoxiao": "zf_xiaoxiao",
  "🇨🇳 🚺 Xiaoyi": "zf_xiaoyi",
  "🇨🇳 🚹 Yunjian": "zm_yunjian",
  "🇨🇳 🚹 Yunxi": "zm_yunxi",
  "🇨🇳 🚹 Yunxia": "zm_yunxia",
  "🇨🇳 🚹 Yunyang": "zm_yunyang",
};

// Default configuration template
const DEFAULT_CONFIG = {
  _version: "1.0.0",
  global_preset: {},
};

// Default preset template
const DEFAULT_PRESET = {
  model: "kokoro",
  params: {
    model_name: "hexgrad/Kokoro-82M",
    voice: "",
    use_gpu: true,
  },
};

// Default chatterbox preset template
const DEFAULT_CHATTERBOX_PRESET = {
  model: "chatterbox",
  params: {
    exaggeration: 0.5,
    cfg_weight: 0.5,
    temperature: 0.8,
    model_name: "just_a_placeholder",
    device: "auto",
    dtype: "float32",
  },
};

// Default RVC params template
const DEFAULT_RVC_PARAMS = {
  pitch_up_key: "0",
  index_path: "",
  pitch_collection_method: "harvest",
  model_path: "",
  index_rate: 0.66,
  filter_radius: 3,
  resample_sr: 0,
  rms_mix_rate: 1,
  protect: 0.33,
};

// Predefined model name options
const MODEL_NAME_OPTIONS = ["hexgrad/Kokoro-82M", "custom/model-path"];

// Device options for chatterbox
const DEVICE_OPTIONS = ["auto", "cpu", "cuda", "mps"];

// Data type options for chatterbox
const DTYPE_OPTIONS = ["float32", "float16", "bfloat16"];

export default function VoiceConfigGenerator() {
  const { toast } = useToast();
  const [config, setConfig] = useState({ ...DEFAULT_CONFIG });
  const [presets, setPresets] = useState<string[]>([]);
  const [currentPreset, setCurrentPreset] = useState("");
  const [includeRvcParams, setIncludeRvcParams] = useState(false);
  const [jsonOutput, setJsonOutput] = useState("");
  const [isRenaming, setIsRenaming] = useState(false);
  const [newPresetName, setNewPresetName] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [testText, setTestText] = useState(
    "Hello, this is a test of the voice synthesis."
  );
  const [isTesting, setIsTesting] = useState(false);
  const [availableChatterboxVoices, setAvailableChatterboxVoices] = useState<
    string[]
  >([]);
  // const [availableKokoroVoices, setAvailableKokoroVoices] = useState<string[]>(
  //   Object.values(VOICE_OPTIONS)
  // );
  const availableKokoroVoices = Object.values(VOICE_OPTIONS);

  const [customVoiceString, setCustomVoiceString] = useState("");
  const [voiceSelections, setVoiceSelections] = useState<
    Record<string, number>
  >({});
  const [isEditingVoice, setIsEditingVoice] = useState(false);

  // Load initial configuration on mount
  useEffect(() => {
    loadConfig();
    loadAvailableVoices();
  }, []);

  // Update JSON output whenever config changes
  useEffect(() => {
    setJsonOutput(JSON.stringify(config, null, 4));
  }, [config]);

  // Update custom voice string when current preset changes
  useEffect(() => {
    if (
      currentPreset &&
      config.global_preset[currentPreset]?.model === "kokoro"
    ) {
      const voiceString =
        config.global_preset[currentPreset]?.params?.voice || "";
      setCustomVoiceString(voiceString);

      // Parse voice string to update voice selections
      const voiceMap: Record<string, number> = {};
      if (voiceString) {
        const voices = voiceString.split(",");
        voices.forEach((voice) => {
          voiceMap[voice] = (voiceMap[voice] || 0) + 1;
        });
      }
      setVoiceSelections(voiceMap);
    }
  }, [currentPreset, config]);

  const loadConfig = async () => {
    setIsLoading(true);
    try {
      const response = await fetch("/api/gradio/open_ai_api_load_presets");
      if (response.ok) {
        const data = await response.json();
        setConfig(data);
        setPresets(Object.keys(data.global_preset || {}));
        if (Object.keys(data.global_preset || {}).length > 0) {
          setCurrentPreset(Object.keys(data.global_preset)[0]);
        }
        toast({
          title: "Configuration loaded",
          description: "Successfully loaded configuration from server.",
        });
      } else {
        throw new Error("Failed to load configuration");
      }
    } catch (error) {
      toast({
        title: "Load failed",
        description: "Failed to load configuration from server.",
        variant: "destructive",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const saveConfig = async () => {
    setIsSaving(true);
    try {
      const response = await fetch("/api/gradio/open_ai_api_save_presets", {
        method: "POST",
        body: JSON.stringify([JSON.stringify(config)]),
      });
      if (response.ok) {
        toast({
          title: "Configuration saved",
          description: "Successfully saved configuration to server.",
        });
      } else {
        throw new Error("Failed to save configuration");
      }
    } catch (error) {
      toast({
        title: "Save failed",
        description: "Failed to save configuration to server.",
        variant: "destructive",
      });
    } finally {
      setIsSaving(false);
    }
  };

  const loadAvailableVoices = async () => {
    try {
      // Load Chatterbox voices only
      const chatterboxResponse = await fetch(
        "/api/gradio/get_chatterbox_voices"
      );
      if (chatterboxResponse.ok) {
        const voices = await chatterboxResponse.json();
        setAvailableChatterboxVoices(voices);
      }
    } catch (error) {
      console.error("Failed to load available voices:", error);
      toast({
        title: "Voice loading failed",
        description: "Failed to load available voices.",
        variant: "destructive",
      });
    }
  };

  const testVoice = async () => {
    if (!currentPreset || !testText.trim()) return;

    setIsTesting(true);
    try {
      await saveConfig();

      const response = await fetch(
        "/api/gradio/open_ai_api_test_voice_preset",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            params: {
              model: "global_preset",
              voice: currentPreset,
              input: testText,
            },
          }),
        }
      );
      if (response.ok) {
        const audioUrl = await response.json();
        const audio = new Audio(audioUrl);
        audio.play();
        toast({
          title: "Voice test",
          description: "Playing generated audio.",
        });
      } else {
        throw new Error("Failed to generate voice");
      }
    } catch (error) {
      toast({
        title: "Test failed",
        description: "Failed to generate voice test.",
        variant: "destructive",
      });
    } finally {
      setIsTesting(false);
    }
  };

  const copySillyTavernVoices = () => {
    const voicesList = presets.join(",");
    navigator.clipboard.writeText(voicesList);
    toast({
      title: "Copied to clipboard",
      description: `Copied ${presets.length} voice names for SillyTavern.`,
    });
  };

  const addPreset = (presetId: string, modelType = "kokoro") => {
    if (!presetId || presets.includes(presetId)) return;

    const newPresets = [...presets, presetId];
    setPresets(newPresets);
    setCurrentPreset(presetId);

    const template =
      modelType === "chatterbox" ? DEFAULT_CHATTERBOX_PRESET : DEFAULT_PRESET;

    setConfig((prev) => {
      const newConfig = { ...prev };
      newConfig.global_preset = {
        ...newConfig.global_preset,
        [presetId]: {
          ...template,
          params: {
            ...template.params,
            voice: modelType === "kokoro" ? "af_heart" : undefined,
          },
        },
      };
      return newConfig;
    });
  };

  const removePreset = (presetId: string) => {
    if (!presetId || !presets.includes(presetId)) return;

    const newPresets = presets.filter((p) => p !== presetId);
    setPresets(newPresets);

    if (currentPreset === presetId) {
      setCurrentPreset(newPresets.length > 0 ? newPresets[0] : "");
    }

    setConfig((prev) => {
      const newConfig = { ...prev };
      const newGlobalPreset = { ...newConfig.global_preset };
      delete newGlobalPreset[presetId];
      newConfig.global_preset = newGlobalPreset;
      return newConfig;
    });
  };

  const updatePresetField = (presetId: string, path: string[], value: any) => {
    setConfig((prev) => {
      const newConfig = { ...prev };
      let target = newConfig.global_preset[presetId];

      // Navigate to the nested property
      for (let i = 0; i < path.length - 1; i++) {
        if (!target[path[i]]) {
          target[path[i]] = {};
        }
        target = target[path[i]];
      }

      // Set the value
      target[path[path.length - 1]] = value;

      // If changing model type, reset model_name and voice to defaults
      if (path.length === 1 && path[0] === "model") {
        if (value === "kokoro") {
          target.params.model_name = "hexgrad/Kokoro-82M";
          target.params.voice = "af_heart";
          // Remove audio_prompt_path if it exists
          delete target.params.audio_prompt_path;

          // Reset voice selections
          setVoiceSelections({ af_heart: 1 });
          setCustomVoiceString("af_heart");
        } else if (value === "chatterbox") {
          target.params.model_name = "just_a_placeholder";
          // Remove voice for chatterbox (it uses audio_prompt_path instead)
          delete target.params.voice;
        }
      }

      return newConfig;
    });
  };

  const toggleRvcParams = () => {
    setIncludeRvcParams(!includeRvcParams);

    if (!includeRvcParams && currentPreset) {
      // Add RVC params when toggling on
      setConfig((prev) => {
        const newConfig = { ...prev };
        if (newConfig.global_preset[currentPreset]) {
          newConfig.global_preset[currentPreset].rvc_params = {
            ...DEFAULT_RVC_PARAMS,
          };
        }
        return newConfig;
      });
    } else if (includeRvcParams && currentPreset) {
      // Remove RVC params when toggling off
      setConfig((prev) => {
        const newConfig = { ...prev };
        if (newConfig.global_preset[currentPreset]) {
          const newPreset = { ...newConfig.global_preset[currentPreset] };
          delete newPreset.rvc_params;
          newConfig.global_preset[currentPreset] = newPreset;
        }
        return newConfig;
      });
    }
  };

  const handleRenamePreset = (oldPresetId: string, newPresetId: string) => {
    if (
      !newPresetId ||
      presets.includes(newPresetId) ||
      oldPresetId === newPresetId
    ) {
      setIsRenaming(false);
      return;
    }

    // Update presets array
    const newPresets = presets.map((p) =>
      p === oldPresetId ? newPresetId : p
    );
    setPresets(newPresets);

    // Update current preset if it's the one being renamed
    if (currentPreset === oldPresetId) {
      setCurrentPreset(newPresetId);
    }

    // Update config
    setConfig((prev) => {
      const newConfig = { ...prev };
      const presetConfig = { ...newConfig.global_preset[oldPresetId] };

      // Update voice parameter if it matches the old preset ID
      if (presetConfig.params && presetConfig.params.voice === oldPresetId) {
        presetConfig.params = {
          ...presetConfig.params,
          voice: newPresetId,
        };
      }

      // Add the renamed preset and remove the old one
      newConfig.global_preset = {
        ...newConfig.global_preset,
        [newPresetId]: presetConfig,
      };
      delete newConfig.global_preset[oldPresetId];

      return newConfig;
    });

    setIsRenaming(false);
  };

  const copyToClipboard = () => {
    navigator.clipboard.writeText(jsonOutput);
    toast({
      title: "Copied to clipboard",
      description: "The JSON configuration has been copied to your clipboard.",
    });
  };

  const handleAddNewPreset = (modelType = "kokoro") => {
    // Find a unique preset ID
    let counter = 1;
    let newPresetId = `${modelType}_preset_${counter}`;
    while (presets.includes(newPresetId)) {
      counter++;
      newPresetId = `${modelType}_preset_${counter}`;
    }

    addPreset(newPresetId, modelType);
  };

  const handleVoiceSelection = (voiceId: string) => {
    if (currentPreset) {
      updatePresetField(currentPreset, ["params", "voice"], voiceId);
      setCustomVoiceString(voiceId);
      setVoiceSelections({ [voiceId]: 1 });
    }
  };

  const handleCustomVoiceStringChange = (value: string) => {
    setCustomVoiceString(value);
    if (currentPreset) {
      updatePresetField(currentPreset, ["params", "voice"], value);

      // Parse voice string to update voice selections
      const voiceMap: Record<string, number> = {};
      if (value) {
        const voices = value.split(",");
        voices.forEach((voice) => {
          voiceMap[voice] = (voiceMap[voice] || 0) + 1;
        });
      }
      setVoiceSelections(voiceMap);
    }
  };

  const handleChatterboxVoiceSelection = (voiceId: string) => {
    if (currentPreset) {
      if (voiceId === "unset") {
        // Remove audio_prompt_path to make it undefined
        setConfig((prev) => {
          const newConfig = { ...prev };
          if (newConfig.global_preset[currentPreset]?.params) {
            delete newConfig.global_preset[currentPreset].params
              .audio_prompt_path;
          }
          return newConfig;
        });
      } else {
        updatePresetField(
          currentPreset,
          ["params", "audio_prompt_path"],
          voiceId
        );
      }
    }
  };

  const incrementVoice = (voice: string) => {
    const newSelections = { ...voiceSelections };
    newSelections[voice] = (newSelections[voice] || 0) + 1;
    setVoiceSelections(newSelections);
    updateVoiceStringFromSelections(newSelections);
  };

  const decrementVoice = (voice: string) => {
    const newSelections = { ...voiceSelections };
    if (newSelections[voice] > 1) {
      newSelections[voice] -= 1;
    } else {
      delete newSelections[voice];
    }
    setVoiceSelections(newSelections);
    updateVoiceStringFromSelections(newSelections);
  };

  const toggleVoice = (voice: string) => {
    const newSelections = { ...voiceSelections };
    if (newSelections[voice]) {
      delete newSelections[voice];
    } else {
      newSelections[voice] = 1;
    }
    setVoiceSelections(newSelections);
    updateVoiceStringFromSelections(newSelections);
  };

  const updateVoiceStringFromSelections = (
    newSelections: Record<string, number>
  ) => {
    const voices: string[] = [];
    Object.entries(newSelections).forEach(([voice, count]) => {
      for (let i = 0; i < count; i++) {
        voices.push(voice);
      }
    });
    const voiceString = voices.join(",");
    setCustomVoiceString(voiceString);
    if (currentPreset) {
      updatePresetField(currentPreset, ["params", "voice"], voiceString);
    }
  };

  const applyVoiceSelections = () => {
    updateVoiceStringFromSelections(voiceSelections);
    setIsEditingVoice(false);
  };

  return (
    <div className="container mx-auto py-8">
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Voice Config Generator</CardTitle>
              <CardDescription>Manage your voice presets</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="flex flex-wrap gap-2 mb-4">
                {presets.map((presetId) => (
                  <div key={presetId} className="flex items-center">
                    {isRenaming && currentPreset === presetId ? (
                      <div className="flex items-center gap-2">
                        <Input
                          value={newPresetName}
                          onChange={(e) => setNewPresetName(e.target.value)}
                          className="h-9 w-40"
                          autoFocus
                          onKeyDown={(e) => {
                            if (e.key === "Enter") {
                              handleRenamePreset(presetId, newPresetName);
                            } else if (e.key === "Escape") {
                              setIsRenaming(false);
                            }
                          }}
                        />
                        <Button
                          size="sm"
                          onClick={() =>
                            handleRenamePreset(presetId, newPresetName)
                          }
                        >
                          Save
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setIsRenaming(false)}
                        >
                          Cancel
                        </Button>
                      </div>
                    ) : (
                      <Button
                        variant={
                          currentPreset === presetId ? "default" : "outline"
                        }
                        onClick={() => setCurrentPreset(presetId)}
                        onDoubleClick={() => {
                          setCurrentPreset(presetId);
                          setNewPresetName(presetId);
                          setIsRenaming(true);
                        }}
                        className="flex items-center gap-2"
                      >
                        {presetId}
                        <div className="flex items-center">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-5 w-5 p-0"
                            onClick={(e) => {
                              e.stopPropagation();
                              setCurrentPreset(presetId);
                              setNewPresetName(presetId);
                              setIsRenaming(true);
                            }}
                          >
                            <svg
                              xmlns="http://www.w3.org/2000/svg"
                              width="15"
                              height="15"
                              viewBox="0 0 24 24"
                              fill="none"
                              stroke="currentColor"
                              strokeWidth="2"
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              className="lucide lucide-pencil"
                            >
                              <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
                            </svg>
                          </Button>
                          <Trash2
                            className="h-4 w-4 text-destructive cursor-pointer"
                            onClick={(e) => {
                              e.stopPropagation();
                              removePreset(presetId);
                            }}
                          />
                        </div>
                      </Button>
                    )}
                  </div>
                ))}
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    onClick={() => handleAddNewPreset("kokoro")}
                  >
                    <Plus className="h-4 w-4 mr-2" />
                    Add Kokoro
                  </Button>
                  <Button
                    variant="outline"
                    onClick={() => handleAddNewPreset("chatterbox")}
                  >
                    <Plus className="h-4 w-4 mr-2" />
                    Add Chatterbox
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>

          {currentPreset && (
            <Card>
              <CardHeader>
                <CardTitle>Configure Preset: {currentPreset}</CardTitle>
                <CardDescription>
                  Set parameters for this voice preset
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="model">Model Type</Label>
                  <Select
                    value={
                      config.global_preset[currentPreset]?.model || "kokoro"
                    }
                    onValueChange={(value) =>
                      updatePresetField(currentPreset, ["model"], value)
                    }
                  >
                    <SelectTrigger id="model">
                      <SelectValue placeholder="Select model type" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="kokoro">Kokoro</SelectItem>
                      <SelectItem value="chatterbox">Chatterbox</SelectItem>
                      <SelectItem value="custom">Custom</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                <Accordion type="single" collapsible defaultValue="params">
                  <AccordionItem value="params">
                    <AccordionTrigger>Basic Parameters</AccordionTrigger>
                    <AccordionContent className="space-y-4 pt-4">
                      {config.global_preset[currentPreset]?.model ===
                      "chatterbox" ? (
                        // Chatterbox parameters
                        <>
                          <div className="space-y-2">
                            <Label htmlFor="chatterbox_voice">
                              Voice (Audio Prompt Path)
                            </Label>
                            <Select
                              value={
                                config.global_preset[currentPreset]?.params
                                  ?.audio_prompt_path || "unset"
                              }
                              onValueChange={handleChatterboxVoiceSelection}
                            >
                              <SelectTrigger id="chatterbox_voice">
                                <SelectValue placeholder="Select voice" />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="unset">Unset</SelectItem>
                                {availableChatterboxVoices.map((voice) => (
                                  <SelectItem key={voice} value={voice}>
                                    {voice.replace(".wav", "")}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="exaggeration">
                              Exaggeration (
                              {config.global_preset[currentPreset]?.params
                                ?.exaggeration || 0.5}
                              )
                            </Label>
                            <Slider
                              id="exaggeration"
                              min={0}
                              max={1}
                              step={0.1}
                              value={[
                                config.global_preset[currentPreset]?.params
                                  ?.exaggeration || 0.5,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "exaggeration"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="cfg_weight">
                              CFG Weight (
                              {config.global_preset[currentPreset]?.params
                                ?.cfg_weight || 0.5}
                              )
                            </Label>
                            <Slider
                              id="cfg_weight"
                              min={0}
                              max={1}
                              step={0.1}
                              value={[
                                config.global_preset[currentPreset]?.params
                                  ?.cfg_weight || 0.5,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "cfg_weight"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="temperature">
                              Temperature (
                              {config.global_preset[currentPreset]?.params
                                ?.temperature || 0.8}
                              )
                            </Label>
                            <Slider
                              id="temperature"
                              min={0}
                              max={2}
                              step={0.1}
                              value={[
                                config.global_preset[currentPreset]?.params
                                  ?.temperature || 0.8,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "temperature"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="model_name">Model Name</Label>
                            <Input
                              id="model_name"
                              value={
                                config.global_preset[currentPreset]?.params
                                  ?.model_name || ""
                              }
                              onChange={(e) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "model_name"],
                                  e.target.value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="device">Device</Label>
                            <Select
                              value={
                                config.global_preset[currentPreset]?.params
                                  ?.device || "auto"
                              }
                              onValueChange={(value) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "device"],
                                  value
                                )
                              }
                            >
                              <SelectTrigger id="device">
                                <SelectValue placeholder="Select device" />
                              </SelectTrigger>
                              <SelectContent>
                                {DEVICE_OPTIONS.map((device) => (
                                  <SelectItem key={device} value={device}>
                                    {device}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="dtype">Data Type</Label>
                            <Select
                              value={
                                config.global_preset[currentPreset]?.params
                                  ?.dtype || "float32"
                              }
                              onValueChange={(value) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "dtype"],
                                  value
                                )
                              }
                            >
                              <SelectTrigger id="dtype">
                                <SelectValue placeholder="Select data type" />
                              </SelectTrigger>
                              <SelectContent>
                                {DTYPE_OPTIONS.map((dtype) => (
                                  <SelectItem key={dtype} value={dtype}>
                                    {dtype}
                                  </SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          </div>
                        </>
                      ) : (
                        // Kokoro parameters
                        <>
                          <div className="space-y-2">
                            <Label htmlFor="voice">Voice</Label>
                            <div className="flex gap-2">
                              {isEditingVoice ? (
                                <div className="flex-1">
                                  <Input
                                    id="custom_voice"
                                    value={customVoiceString}
                                    onChange={(e) =>
                                      handleCustomVoiceStringChange(
                                        e.target.value
                                      )
                                    }
                                    placeholder="e.g., af_heart,af_heart,af_aoede"
                                  />
                                </div>
                              ) : (
                                <Select
                                  value={customVoiceString || ""}
                                  onValueChange={handleVoiceSelection}
                                >
                                  <SelectTrigger id="voice" className="flex-1">
                                    <SelectValue placeholder="Select voice" />
                                  </SelectTrigger>
                                  <SelectContent>
                                    {/* {availableKokoroVoices.map((voice) => (
                                      <SelectItem key={voice} value={voice}>
                                        {voice}
                                      </SelectItem>
                                    ))} */}
                                    {/* VOICE_OPTIONS */}
                                    {Object.entries(VOICE_OPTIONS).map(
                                      ([label, value]) => (
                                        <SelectItem key={value} value={value}>
                                          {label}
                                        </SelectItem>
                                      )
                                    )}
                                    {/* current selection */}
                                    {customVoiceString &&
                                      !availableKokoroVoices.includes(
                                        customVoiceString
                                      ) && (
                                        <SelectItem value={customVoiceString}>
                                          {customVoiceString}
                                        </SelectItem>
                                      )}
                                  </SelectContent>
                                </Select>
                              )}
                              <Popover>
                                <PopoverTrigger asChild>
                                  <Button variant="outline" size="icon">
                                    <Edit className="h-4 w-4" />
                                  </Button>
                                </PopoverTrigger>
                                <PopoverContent className="w-80">
                                  <div className="space-y-4">
                                    <h4 className="font-medium">Voice Mixer</h4>
                                    <p className="text-sm text-muted-foreground">
                                      Select multiple voices and adjust their
                                      counts.
                                    </p>

                                    <Tabs defaultValue="select">
                                      <TabsList className="grid w-full grid-cols-2">
                                        <TabsTrigger value="select">
                                          Select
                                        </TabsTrigger>
                                        <TabsTrigger value="custom">
                                          Custom
                                        </TabsTrigger>
                                      </TabsList>
                                      <TabsContent
                                        value="select"
                                        className="space-y-4"
                                      >
                                        <div className="max-h-60 overflow-y-auto space-y-2">
                                          {availableKokoroVoices.map(
                                            (voice) => (
                                              <div
                                                key={voice}
                                                className="flex items-center justify-between"
                                              >
                                                <div className="flex items-center gap-2">
                                                  <Button
                                                    variant="outline"
                                                    size="icon"
                                                    className="h-6 w-6"
                                                    onClick={() =>
                                                      toggleVoice(voice)
                                                    }
                                                  >
                                                    {voiceSelections[voice] ? (
                                                      <Check className="h-3 w-3" />
                                                    ) : (
                                                      <Plus className="h-3 w-3" />
                                                    )}
                                                  </Button>
                                                  <span className="text-sm">
                                                    {voice}
                                                  </span>
                                                </div>
                                                {voiceSelections[voice] && (
                                                  <div className="flex items-center gap-1">
                                                    <Button
                                                      variant="outline"
                                                      size="icon"
                                                      className="h-6 w-6"
                                                      onClick={() =>
                                                        decrementVoice(voice)
                                                      }
                                                    >
                                                      <Minus className="h-3 w-3" />
                                                    </Button>
                                                    <span className="text-sm w-5 text-center">
                                                      {voiceSelections[voice]}
                                                    </span>
                                                    <Button
                                                      variant="outline"
                                                      size="icon"
                                                      className="h-6 w-6"
                                                      onClick={() =>
                                                        incrementVoice(voice)
                                                      }
                                                    >
                                                      <Plus className="h-3 w-3" />
                                                    </Button>
                                                  </div>
                                                )}
                                              </div>
                                            )
                                          )}
                                        </div>
                                      </TabsContent>
                                      <TabsContent value="custom">
                                        <div className="space-y-2">
                                          <Label htmlFor="custom_voice_string">
                                            Custom Voice String
                                          </Label>
                                          <Input
                                            id="custom_voice_string"
                                            value={customVoiceString}
                                            onChange={(e) =>
                                              handleCustomVoiceStringChange(
                                                e.target.value
                                              )
                                            }
                                            placeholder="e.g., af_heart,af_heart,af_aoede"
                                          />
                                          <p className="text-xs text-muted-foreground">
                                            Enter comma-separated voice IDs.
                                            Repeat IDs to increase their weight.
                                          </p>
                                        </div>
                                      </TabsContent>
                                    </Tabs>

                                    <div className="pt-2">
                                      <h5 className="text-sm font-medium mb-2">
                                        Selected Voices:
                                      </h5>
                                      <div className="flex flex-wrap gap-1">
                                        {Object.entries(voiceSelections).map(
                                          ([voice, count]) => (
                                            <Badge
                                              key={voice}
                                              variant="secondary"
                                            >
                                              {voice}{" "}
                                              {count > 1 && `(x${count})`}
                                            </Badge>
                                          )
                                        )}
                                        {Object.keys(voiceSelections).length ===
                                          0 && (
                                          <span className="text-sm text-muted-foreground">
                                            No voices selected
                                          </span>
                                        )}
                                      </div>
                                    </div>

                                    <div className="flex justify-end gap-2">
                                      <Button
                                        variant="outline"
                                        size="sm"
                                        onClick={() => setVoiceSelections({})}
                                      >
                                        Clear
                                      </Button>
                                      {/* <Button
                                        size="sm"
                                        onClick={applyVoiceSelections}
                                      >
                                        Apply
                                      </Button> */}
                                    </div>
                                  </div>
                                </PopoverContent>
                              </Popover>
                              <Button
                                variant="outline"
                                size="icon"
                                onClick={() =>
                                  setIsEditingVoice(!isEditingVoice)
                                }
                              >
                                {isEditingVoice ? (
                                  <Check className="h-4 w-4" />
                                ) : (
                                  <Edit className="h-4 w-4" />
                                )}
                              </Button>
                            </div>
                            {customVoiceString &&
                              customVoiceString.includes(",") && (
                                <div className="mt-1">
                                  <div className="flex flex-wrap gap-1">
                                    {Object.entries(voiceSelections).map(
                                      ([voice, count]) => (
                                        <Badge key={voice} variant="secondary">
                                          {voice} {count > 1 && `(x${count})`}
                                        </Badge>
                                      )
                                    )}
                                  </div>
                                </div>
                              )}
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="model_name">Model Name</Label>
                            <Select
                              value={
                                config.global_preset[currentPreset]?.params
                                  ?.model_name || ""
                              }
                              onValueChange={(value) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "model_name"],
                                  value
                                )
                              }
                            >
                              <SelectTrigger id="model_name">
                                <SelectValue placeholder="Select model name" />
                              </SelectTrigger>
                              <SelectContent>
                                {MODEL_NAME_OPTIONS.map((model) => (
                                  <SelectItem key={model} value={model}>
                                    {model}
                                  </SelectItem>
                                ))}
                                <SelectItem value="custom">Custom</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="flex items-center space-x-2">
                            <Switch
                              id="use_gpu"
                              checked={
                                config.global_preset[currentPreset]?.params
                                  ?.use_gpu || false
                              }
                              onCheckedChange={(checked) =>
                                updatePresetField(
                                  currentPreset,
                                  ["params", "use_gpu"],
                                  checked
                                )
                              }
                            />
                            <Label htmlFor="use_gpu">Use GPU</Label>
                          </div>
                        </>
                      )}
                    </AccordionContent>
                  </AccordionItem>

                  <AccordionItem value="rvc">
                    <AccordionTrigger>
                      <div className="flex items-center">
                        <span>RVC Parameters</span>
                        <Switch
                          className="ml-2"
                          checked={includeRvcParams}
                          onCheckedChange={toggleRvcParams}
                          onClick={(e) => e.stopPropagation()}
                        />
                      </div>
                    </AccordionTrigger>
                    <AccordionContent className="space-y-4 pt-4">
                      {includeRvcParams && (
                        <>
                          <div className="space-y-2">
                            <Label htmlFor="pitch_up_key">
                              Pitch Up Key (
                              {config.global_preset[currentPreset]?.rvc_params
                                ?.pitch_up_key || "0"}
                              )
                            </Label>
                            <Slider
                              id="pitch_up_key"
                              min={-12}
                              max={12}
                              step={1}
                              value={[
                                Number.parseInt(
                                  config.global_preset[currentPreset]
                                    ?.rvc_params?.pitch_up_key || "0"
                                ),
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "pitch_up_key"],
                                  value.toString()
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="index_path">Index Path</Label>
                            <Input
                              id="index_path"
                              value={
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.index_path || ""
                              }
                              onChange={(e) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "index_path"],
                                  e.target.value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="pitch_collection_method">
                              Pitch Collection Method
                            </Label>
                            <Select
                              value={
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.pitch_collection_method || "harvest"
                              }
                              onValueChange={(value) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "pitch_collection_method"],
                                  value
                                )
                              }
                            >
                              <SelectTrigger id="pitch_collection_method">
                                <SelectValue placeholder="Select method" />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="harvest">Harvest</SelectItem>
                                <SelectItem value="dio">DIO</SelectItem>
                                <SelectItem value="crepe">Crepe</SelectItem>
                              </SelectContent>
                            </Select>
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="model_path">Model Path</Label>
                            <Input
                              id="model_path"
                              value={
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.model_path || ""
                              }
                              onChange={(e) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "model_path"],
                                  e.target.value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="index_rate">
                              Index Rate (
                              {config.global_preset[currentPreset]?.rvc_params
                                ?.index_rate || 0.66}
                              )
                            </Label>
                            <Slider
                              id="index_rate"
                              min={0}
                              max={1}
                              step={0.01}
                              value={[
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.index_rate || 0.66,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "index_rate"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="filter_radius">
                              Filter Radius (
                              {config.global_preset[currentPreset]?.rvc_params
                                ?.filter_radius || 3}
                              )
                            </Label>
                            <Slider
                              id="filter_radius"
                              min={0}
                              max={10}
                              step={1}
                              value={[
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.filter_radius || 3,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "filter_radius"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="resample_sr">Resample SR</Label>
                            <Input
                              id="resample_sr"
                              type="number"
                              value={
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.resample_sr || 0
                              }
                              onChange={(e) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "resample_sr"],
                                  Number.parseInt(e.target.value)
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="rms_mix_rate">
                              RMS Mix Rate (
                              {config.global_preset[currentPreset]?.rvc_params
                                ?.rms_mix_rate || 1}
                              )
                            </Label>
                            <Slider
                              id="rms_mix_rate"
                              min={0}
                              max={1}
                              step={0.01}
                              value={[
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.rms_mix_rate || 1,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "rms_mix_rate"],
                                  value
                                )
                              }
                            />
                          </div>

                          <div className="space-y-2">
                            <Label htmlFor="protect">
                              Protect (
                              {config.global_preset[currentPreset]?.rvc_params
                                ?.protect || 0.33}
                              )
                            </Label>
                            <Slider
                              id="protect"
                              min={0}
                              max={1}
                              step={0.01}
                              value={[
                                config.global_preset[currentPreset]?.rvc_params
                                  ?.protect || 0.33,
                              ]}
                              onValueChange={([value]) =>
                                updatePresetField(
                                  currentPreset,
                                  ["rvc_params", "protect"],
                                  value
                                )
                              }
                            />
                          </div>
                        </>
                      )}
                    </AccordionContent>
                  </AccordionItem>
                </Accordion>
              </CardContent>
            </Card>
          )}
        </div>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="flex justify-between items-center">
                <span>JSON Output</span>
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={copyToClipboard}>
                    <Clipboard className="h-4 w-4 mr-2" />
                    Copy JSON
                  </Button>
                  <Button
                    onClick={copySillyTavernVoices}
                    variant="outline"
                    size="sm"
                  >
                    <Copy className="h-4 w-4 mr-2" />
                    Copy Voices
                  </Button>
                  <Button
                    onClick={loadConfig}
                    disabled={isLoading}
                    variant="outline"
                    size="sm"
                  >
                    <RefreshCw
                      className={`h-4 w-4 mr-2 ${isLoading ? "animate-spin" : ""}`}
                    />
                    Load
                  </Button>
                  <Button onClick={saveConfig} disabled={isSaving} size="sm">
                    <Save className="h-4 w-4 mr-2" />
                    {isSaving ? "Saving..." : "Save"}
                  </Button>
                </div>
              </CardTitle>
              <CardDescription>Generated configuration</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="test-text">Test Voice</Label>
                <div className="flex gap-2">
                  <Input
                    id="test-text"
                    value={testText}
                    onChange={(e) => setTestText(e.target.value)}
                    placeholder="Enter text to test..."
                    className="flex-1"
                  />
                  <Button
                    onClick={testVoice}
                    disabled={isTesting || !currentPreset}
                  >
                    <Play
                      className={`h-4 w-4 mr-2 ${isTesting ? "animate-spin" : ""}`}
                    />
                    {isTesting ? "Testing..." : "Save & Test"}
                  </Button>
                </div>
              </div>
              <Textarea
                className="font-mono h-[400px] resize-none"
                value={jsonOutput}
                readOnly
              />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>API Sample Code</CardTitle>
              <CardDescription>Examples for using the API</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label>Python (OpenAI Compatible)</Label>
                <div className="bg-gray-100 p-4 rounded-md font-mono text-sm">
                  <pre>{`from openai import OpenAI

client = OpenAI(
    api_key="sk-1234567890", 
    base_url="http://localhost:7778/v1"
)

with client.audio.speech.with_streaming_response.create(
    model="global_preset",
    voice="${currentPreset || "your_preset_name"}",
    input="Today is a wonderful day to build something people love!",
) as response:
    audio = response.read()
    with open("audio.mp3", "wb") as f:
        f.write(audio)`}</pre>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
