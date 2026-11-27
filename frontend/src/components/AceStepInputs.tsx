import React, { useState } from "react";
import { AceStepParams, initialAceStepParams } from "../tabs/AceStepParams";
import { PromptTextArea } from "./PromptTextArea";
import { HandleChange } from "../types/HandleChange";
import { ParameterSlider } from "./GenericSlider";
import { Label } from "./ui/label";
import { RadioGroup, RadioGroupItem } from "./ui/radio-group";
import { Switch } from "./ui/switch";
import { ResetButton } from "./ResetButton";
import { Input } from "./ui/input";
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "./ui/accordion";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./ui/card";
import { TrashIcon } from "lucide-react";
import { AudioInput } from "./AudioComponents";

interface AceStepInputsProps {
  aceStepParams: AceStepParams;
  handleChange: HandleChange;
  setAceStepParams: React.Dispatch<React.SetStateAction<AceStepParams>>;
}

export const AceStepInputs: React.FC<AceStepInputsProps> = ({
  aceStepParams,
  handleChange,
  setAceStepParams,
}) => {
  const [manualSeeds, setManualSeeds] = useState<string>(
    aceStepParams.manual_seeds || ""
  );

  const handleCheckboxChange = (name: string) => (checked: boolean) => {
    setAceStepParams((prev) => ({
      ...prev,
      [name]: checked,
    }));
  };

  const handleRadioChange = (name: string) => (value: string) => {
    setAceStepParams((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleManualSeedsChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setManualSeeds(e.target.value);
    setAceStepParams((prev) => ({
      ...prev,
      manual_seeds: e.target.value,
    }));
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Tags</CardTitle>
            <CardDescription>
              Support tags, descriptions, and scene. Use commas to separate
              different tags.
              <br />
              tags and lyrics examples are from ai music generation community
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <PromptTextArea
              params={aceStepParams}
              handleChange={handleChange}
              label=""
              name="tags"
              rows={3}
              placeholder="funk, pop, soul, rock, melodic, guitar, drums, bass, keyboard, percussion, 105 BPM, energetic, upbeat, groovy, vibrant, dynamic"
            />
            {/* Tags as breadcrumbs */}
            <div className="flex flex-wrap gap-2">
              {aceStepParams.tags.split(",").map((tag) => (
                <div
                  key={tag}
                  className="bg-muted text-muted-foreground rounded px-2 py-1 text-xs flex items-center gap-2"
                >
                  {tag}
                  {/* delete icon */}
                  <button
                    className="hover:opacity-50"
                    onClick={() => {
                      setAceStepParams((prev) => ({
                        ...prev,
                        tags: prev.tags
                          .split(",")
                          .filter((t) => t !== tag)
                          .join(","),
                      }));
                    }}
                  >
                    <TrashIcon className="h-4 w-4" />
                  </button>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardTitle>Lyrics</CardTitle>
            <CardDescription>
              Support lyric structure tags like [verse], [chorus], and [bridge]
              to separate different parts of the lyrics.
              <br />
              Use [instrumental] or [inst] to generate instrumental music. Not
              support genre structure tag in lyrics.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <PromptTextArea
              params={aceStepParams}
              handleChange={handleChange}
              label=""
              name="lyrics"
              rows={32}
              placeholder="[verse]
Neon lights they flicker bright
City hums in dead of night

[chorus]
Turn it up and let it flow
Feel the fire let it grow"
            />
          </CardContent>
        </Card>
      </div>

      <div className="flex flex-col gap-4">
        <Card>
          <CardHeader className="flex flex-row justify-between pb-2">
            <CardTitle>Audio Duration</CardTitle>
            <CardDescription>
              -1 means random duration (30 ~ 240).
            </CardDescription>
          </CardHeader>
          <CardContent>
            <ParameterSlider
              params={aceStepParams}
              onChange={handleChange}
              label=""
              name="audio_duration"
              min="-1"
              max="240"
              step="1"
              decimals={0}
            />
          </CardContent>
        </Card>

        <div>
          <Label className="mb-2 block">Audio-to-Audio</Label>
          <CardDescription className="mb-2">
            Use a reference audio to guide the generation.
          </CardDescription>
          <div className="flex items-center space-x-2 mb-4">
            <Switch
              id="audio2audio_enable"
              checked={aceStepParams.audio2audio_enable}
              onCheckedChange={handleCheckboxChange("audio2audio_enable")}
            />
            <div>
              <Label htmlFor="audio2audio_enable">Enable Audio-to-Audio</Label>
              <CardDescription>
                When enabled, the model will use the reference audio to guide
                generation.
              </CardDescription>
            </div>
          </div>

          {aceStepParams.audio2audio_enable && (
            <>
              <ParameterSlider
                params={aceStepParams}
                onChange={handleChange}
                label="Reference Audio Strength"
                name="ref_audio_strength"
                min="0"
                max="1"
                step="0.01"
                decimals={2}
              />
              <AudioInput
                url={aceStepParams.ref_audio_input}
                callback={(file) => {
                  setAceStepParams((prev) => ({
                    ...prev,
                    ref_audio_input: file,
                  }));
                }}
                label="Reference Audio"
                filter={[]}
              />
            </>
          )}
        </div>

        <ResetButton
          params={aceStepParams}
          setParams={setAceStepParams}
          initialParams={initialAceStepParams}
        />
        <Accordion type="single" collapsible defaultValue="basic">
          <AccordionItem value="basic">
            <AccordionTrigger>Basic Settings</AccordionTrigger>
            <AccordionContent>
              <div className="flex flex-col gap-4">
                <div className="grid grid-cols-4 cell gap-2 items-end">
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Infer Steps"
                    name="infer_steps"
                    min="1"
                    max="1000"
                    step="1"
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Guidance Scale"
                    name="guidance_scale"
                    min="0"
                    max="200"
                    step="0.1"
                    decimals={1}
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Guidance Scale Text"
                    name="guidance_scale_text"
                    min="0"
                    max="10"
                    step="0.1"
                    decimals={1}
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Guidance Scale Lyric"
                    name="guidance_scale_lyric"
                    min="0"
                    max="10"
                    step="0.1"
                    decimals={1}
                    orientation="vertical"
                    className="h-40"
                  />
                </div>

                <CardDescription className="mb-2">
                  <strong>Guidance Scale:</strong> When guidance_scale_lyric{" "}
                  {`>`} 1 and guidance_scale_text {`>`} 1, the guidance scale
                  will not be applied.
                  <br />
                  <strong>Guidance Scale Text:</strong> Guidance scale for text
                  condition. It can only apply to cfg.
                  <br />
                  Set guidance_scale_text=5.0, guidance_scale_lyric=1.5 for
                  start.
                </CardDescription>

                <div>
                  <Label className="mb-2 block">
                    Manual Seeds (default None)
                  </Label>
                  <Input
                    value={manualSeeds}
                    onChange={handleManualSeedsChange}
                    placeholder="1,2,3,4"
                  />
                </div>
              </div>
            </AccordionContent>
          </AccordionItem>

          <AccordionItem value="advanced">
            <AccordionTrigger>Advanced Settings</AccordionTrigger>
            <AccordionContent>
              <div className="flex flex-col gap-4">
                <div>
                  <Label className="mb-2 block">Scheduler Type</Label>
                  <CardDescription className="mb-2">
                    Scheduler type for the generation. euler is recommended.
                    heun will take more time.
                  </CardDescription>
                  <RadioGroup
                    value={aceStepParams.scheduler_type}
                    onValueChange={handleRadioChange("scheduler_type")}
                    className="flex gap-4"
                  >
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="euler" id="euler" />
                      <Label htmlFor="euler">Euler</Label>
                    </div>
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="heun" id="heun" />
                      <Label htmlFor="heun">Heun</Label>
                    </div>
                  </RadioGroup>
                </div>

                <div>
                  <Label className="mb-2 block">CFG Type</Label>
                  <CardDescription className="mb-2">
                    CFG type for the generation. apg is recommended. cfg and
                    cfg_star are almost the same.
                  </CardDescription>
                  <RadioGroup
                    value={aceStepParams.cfg_type}
                    onValueChange={handleRadioChange("cfg_type")}
                    className="flex gap-4 flex-wrap"
                  >
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="cfg" id="cfg" />
                      <Label htmlFor="cfg">CFG</Label>
                    </div>
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="apg" id="apg" />
                      <Label htmlFor="apg">APG</Label>
                    </div>
                    <div className="flex items-center space-x-2">
                      <RadioGroupItem value="cfg_star" id="cfg_star" />
                      <Label htmlFor="cfg_star">CFG Star</Label>
                    </div>
                  </RadioGroup>
                </div>

                <div className="flex flex-col gap-2">
                  <div className="flex items-center space-x-2">
                    <Switch
                      id="use_erg_for_tag"
                      checked={aceStepParams.use_erg_for_tag}
                      onCheckedChange={handleCheckboxChange("use_erg_for_tag")}
                    />
                    <div>
                      <Label htmlFor="use_erg_for_tag">Use ERG for Tag</Label>
                      <CardDescription>
                        It will multiply a temperature to the attention to make
                        a weaker tag condition and make better diversity.
                      </CardDescription>
                    </div>
                  </div>

                  <div className="flex items-center space-x-2">
                    <Switch
                      id="use_erg_for_lyric"
                      checked={aceStepParams.use_erg_for_lyric}
                      onCheckedChange={handleCheckboxChange(
                        "use_erg_for_lyric"
                      )}
                    />
                    <div>
                      <Label htmlFor="use_erg_for_lyric">
                        Use ERG for Lyric
                      </Label>
                      <CardDescription>
                        The same but apply to lyric encoder's attention.
                      </CardDescription>
                    </div>
                  </div>

                  <div className="flex items-center space-x-2">
                    <Switch
                      id="use_erg_for_diffusion"
                      checked={aceStepParams.use_erg_for_diffusion}
                      onCheckedChange={handleCheckboxChange(
                        "use_erg_for_diffusion"
                      )}
                    />
                    <div>
                      <Label htmlFor="use_erg_for_diffusion">
                        Use ERG for Diffusion
                      </Label>
                      <CardDescription>
                        The same but apply to diffusion model's attention.
                      </CardDescription>
                    </div>
                  </div>
                </div>
                <div className="grid grid-cols-4 gap-2 items-end cell">
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Granularity Scale"
                    name="granularity_scale"
                    min="-100"
                    max="100"
                    step="0.1"
                    decimals={1}
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Guidance Interval"
                    name="guidance_interval"
                    min="0"
                    max="1"
                    step="0.01"
                    decimals={2}
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Guidance Interval Decay"
                    name="guidance_interval_decay"
                    min="0"
                    max="1"
                    step="0.01"
                    decimals={2}
                    orientation="vertical"
                    className="h-40"
                  />
                  <ParameterSlider
                    params={aceStepParams}
                    onChange={handleChange}
                    label="Min Guidance Scale"
                    name="min_guidance_scale"
                    min="0"
                    max="200"
                    step="0.1"
                    decimals={1}
                    orientation="vertical"
                    className="h-40"
                  />
                </div>
                <CardDescription className="mt-2">
                  <strong>Granularity Scale:</strong> Higher values can reduce
                  artifacts.
                  <br />
                  <strong>Guidance Interval:</strong> 0.5 means only apply
                  guidance in the middle steps (0.25 * infer_steps to 0.75 *
                  infer_steps).
                  <br />
                  <strong>Guidance Interval Decay:</strong> Guidance scale will
                  decay from guidance_scale to min_guidance_scale in the
                  interval. 0.0 means no decay.
                  <br />
                  <strong>Min Guidance Scale:</strong> Min guidance scale for
                  guidance interval decay's end scale.
                </CardDescription>
                <div>
                  <Label className="mb-2 block">OSS Steps</Label>
                  <CardDescription className="mb-2">
                    Optimal Steps for the generation. But not test well.
                  </CardDescription>
                  <PromptTextArea
                    params={aceStepParams}
                    handleChange={handleChange}
                    label=""
                    name="oss_steps"
                    placeholder="16, 29, 52, 96, 129, 158, 172, 183, 189, 200"
                  />
                </div>
              </div>
            </AccordionContent>
          </AccordionItem>
        </Accordion>
      </div>
    </div>
  );
};
