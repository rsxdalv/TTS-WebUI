import React from "react";
import { MMSParams, MMSResult, initialMMSParams } from "../tabs/MMSParams";
import { HandleChange } from "../types/HandleChange";
import { PromptTextArea } from "./PromptTextArea";
import { ParameterSlider } from "./GenericSlider";
import { MMS_LANGUAGE_DATA } from "./MMS_LANGUAGE_DATA";
import { SeedInput } from "./SeedInput";
import { Button } from "./ui/button";
import { Check, ChevronsUpDown } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Label } from "./ui/label";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "./ui/collapsible";

const ResetButton = <T extends {}>({
  params,
  setParams,
  initialParams,
}: {
  params: T;
  setParams: React.Dispatch<React.SetStateAction<T>>;
  initialParams: T;
}) => (
  <Button
    variant="outline"
    onClick={() => setParams({ ...params, ...initialParams })}
  >
    Reset Parameters
  </Button>
);

const MMS_LANGUAGE_OPTIONS = MMS_LANGUAGE_DATA.map(
  ([visual_model_language, language]) => ({
    label: visual_model_language,
    value: language,
  })
);

export function ComboboxDemo({ value, setValue }) {
  const [open, setOpen] = React.useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-[200px] justify-between"
        >
          {value
            ? MMS_LANGUAGE_OPTIONS.find(
                (framework) => framework.value === value
              )?.label
            : "Select framework..."}
          <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[200px] p-0">
        <Command>
          <CommandInput placeholder="Search framework..." />
          <CommandList>
            <CommandEmpty>No framework found.</CommandEmpty>
            <CommandGroup>
              {MMS_LANGUAGE_OPTIONS.map((framework) => (
                <CommandItem
                  key={framework.value}
                  value={framework.value}
                  keywords={[framework.label]}
                  onSelect={(currentValue) => {
                    setValue(currentValue === value ? "" : currentValue);
                    setOpen(false);
                  }}
                >
                  <Check
                    className={cn(
                      "mr-2 h-4 w-4",
                      value === framework.value ? "opacity-100" : "opacity-0"
                    )}
                  />
                  {framework.label}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

const MemoizedComboboxDemo = React.memo(ComboboxDemo);

export const MMSInputs = ({
  mmsParams: params,
  handleChange,
  setMmsParams,
}: {
  mmsParams: MMSParams;
  handleChange: HandleChange;
  setMmsParams: React.Dispatch<React.SetStateAction<MMSParams>>;
}) => (
  <div className="flex gap-x-6 w-full justify-center">
    <div className="flex flex-col gap-y-2 w-1/2">
      <PromptTextArea
        params={params}
        handleChange={handleChange}
        label="Text"
        name="text"
      />
      <Collapsible>
        <CollapsibleTrigger className="hover:underline flex items-center">
          Description
          <ChevronsUpDown className="ml-2 h-4 w-4" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <p>
            The MMS-TTS checkpoints are trained on lower-cased, un-punctuated
            text. By default, the VitsTokenizer normalizes the inputs by
            removing any casing and punctuation, to avoid passing out-of-
            vocabulary characters to the model. Hence, the model is agnostic to
            casing and punctuation, so these should be avoided in the text
            prompt.
          </p>
          <p>
            For certain languages with non-Roman alphabets, such as Arabic,
            Mandarin or Hindi, the uroman perl package is required to
            pre-process the text inputs to the Roman alphabet.
          </p>
        </CollapsibleContent>
      </Collapsible>
    </div>

    <div className="flex flex-col gap-y-2">
      <div className="flex gap-2 items-center">
        <Label>Language:</Label>
        <MemoizedComboboxDemo
          value={params.language}
          setValue={(value) =>
            handleChange({ target: { name: "language", value } })
          }
        />
      </div>

      <div className="grid grid-cols-3 gap-2 cell">
        <ParameterSlider
          params={params}
          onChange={handleChange}
          label={
            <span title="Larger values give faster synthesised speech.">
              Speaking Rate
            </span>
          }
          decimals={1}
          name="speaking_rate"
          min="0.1"
          max="10.0"
          step="0.1"
          orientation="vertical"
          className="h-40"
        />

        <ParameterSlider
          params={params}
          onChange={handleChange}
          label={
            <span title="How random the speech prediction is. Larger values create more variation in the predicted speech.">
              Noise Scale
            </span>
          }
          decimals={2}
          name="noise_scale"
          min="-2.5"
          max="2.5"
          step="0.05"
          orientation="vertical"
          className="h-40"
        />

        <ParameterSlider
          params={params}
          onChange={handleChange}
          label={
            <span title="How random the duration prediction is. Larger values create more variation in the predicted durations.">
              Noise Scale Duration
            </span>
          }
          decimals={2}
          name="noise_scale_duration"
          min="-1.0"
          max="2"
          step="0.05"
          orientation="vertical"
          className="h-40"
        />
      </div>

      <SeedInput params={params} handleChange={handleChange} />

      <ResetButton
        params={params}
        setParams={setMmsParams}
        initialParams={initialMMSParams}
      />
    </div>
  </div>
);
