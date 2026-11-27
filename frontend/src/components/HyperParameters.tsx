import React from "react";
import useLocalStorage from "../hooks/useLocalStorage";
import { extractTexts, initialHyperParams } from "../data/hyperParamsUtils";
import { applySeed } from "../data/applySeed";
import { useInterrupt } from "../hooks/useInterrupt";
import { Progress } from "./ui/progress";
import { ParameterSlider } from "./GenericSlider";
import { Switch } from "./ui/switch";
import { Label } from "./ui/label";
import { cn } from "../lib/utils";
import { Button } from "./ui/button";
import { ArrowRightIcon, BanIcon, CheckIcon, SendHorizontalIcon, XIcon } from "lucide-react";

const getParams = <T extends { seed: number }>(
  texts: string[],
  iterations: number,
  params: T
) =>
  Array(iterations)
    .fill(0)
    .flatMap((_, iteration) =>
      texts.map((text) => ({
        ...params,
        text,
        seed: params.seed + iteration,
      }))
    );

const progressMiddleware = (setProgress, max) =>
  function* (next) {
    setProgress({ current: 0, max });
    for (const params of next) {
      yield params;
      setProgress(({ current, max }) => ({
        current: current + 1,
        max,
        params,
      }));
    }
    setProgress({ current: 0, max: 0 });
  };

const interruptionMiddleware = (interrupted) =>
  function* (next) {
    for (const params of next) {
      if (interrupted.current) {
        break;
      }
      yield params;
    }
  };

const logMiddleware = () =>
  function* (next) {
    for (const params of next) {
      console.log("Before generation", params);
      yield params;
      console.log("After generation", params);
    }
  };

const pipe =
  (...middlewares) =>
  (iterator) =>
    middlewares.reduceRight((next, middleware) => middleware(next), iterator);

const finalPipe = async (
  consumerFn,
  sourceIterator,
  ...iteratorMiddlewares
) => {
  for (const params of pipe(...iteratorMiddlewares)(sourceIterator)) {
    await consumerFn(params);
  }
};

const batchRunner2 = (interrupted, consume, paramsList, setProgress) =>
  finalPipe(
    consume,
    paramsList,
    progressMiddleware(setProgress, paramsList.length),
    interruptionMiddleware(interrupted),
    logMiddleware()
  );

export const HyperParameters = ({
  consumer,
  prefix,
  genParams,
  className,
}: {
  genParams: { text: string; seed: number; use_random_seed?: boolean };
  consumer: (x: any) => Promise<any>;
  prefix?: string;
  className?: string;
}) => {
  const [params, setParams] = useLocalStorage<typeof initialHyperParams>(
    prefix + "HyperParams",
    initialHyperParams
  );
  const { interrupted, resetInterrupt, interrupt } = useInterrupt();
  const [progress, setProgress] = React.useState({ current: 0, max: 0 });

  const generate = resetInterrupt(() =>
    batchRunner2(
      interrupted,
      consumer,
      getParams(
        extractTexts(genParams.text, params),
        params.iterations,
        applySeed(genParams)
      ),
      setProgress
    )
  );

  return (
    <div
      className={cn(
        "flex flex-col gap-y-2 cell flex-shrink-0",
        className
      )}
    >
      <label className="text-base">Generation Controls:</label>
      <ParameterSlider
        params={params}
        onChange={(event) => {
          setParams({
            ...params,
            iterations: Number(event.target.value),
          });
        }}
        label="Iterations"
        name="iterations"
        min="1"
        max="10"
        step="1"
      />
      <div className="flex gap-x-2 items-center">
        <Switch
          id="splitByLines"
          checked={params.splitByLines}
          onCheckedChange={(value) =>
            setParams({
              ...params,
              splitByLines: value,
            })
          }
        />
        <Label htmlFor="splitByLines">Each line as a separate prompt</Label>
      </div>
      <ProgressStatus value={progress.current} max={progress.max || 1} />
      <div className="flex gap-2">
        <Button className="p-2" variant="destructive" onClick={interrupt}>
          {interrupted.current ? "Interrupted..." : "Interrupt"}
          <BanIcon className="ml-2 h-5 w-5 flex-shrink-0" />
        </Button>
        <Button className="p-2" variant="default" onClick={generate}>
          Generate
          <SendHorizontalIcon className="ml-2 h-5 w-5 flex-shrink-0" />
        </Button>
      </div>
    </div>
  );
};

const ProgressStatus = ({ value, max }: { value: number; max: number }) => (
  <div className="flex gap-x-2 items-center text-sm">
    <label>Progress:</label>
    <Progress value={value} max={max} className="h-4 w-2/3" />
    <span>{value}&nbsp;of&nbsp;{max}</span>
  </div>
);
