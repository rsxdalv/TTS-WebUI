import ThumbsUpIcon from "@material-design-icons/svg/filled/thumb_up.svg";
import ThumbsDownIcon from "@material-design-icons/svg/filled/thumb_down.svg";
import React from "react";
import { MUIIcon } from "./mini/MUIIcon";

export const Vote = ({
  vote,
  setVote,
  voteCount,
}: {
  vote: number;
  setVote: (vote: number) => void;
  voteCount: number;
}) => (
  <div className="flex items-center justify-center gap-2">
    <button
      className={`text-xl text-gray-900 select-none hover:opacity-50 ${
        vote === 1
          ? "text-green-500 hover:text-green-900 disabled:opacity-100"
          : "hover:text-green-500 opacity-20"
      }`}
      onClick={() => setVote(1)}
      title={`Like this voice`}
      disabled={vote === 1}
    >
      <MUIIcon icon={ThumbsUpIcon} alt="like" />
    </button>
    <span className="text-xl text-gray-900">{voteCount}</span>
    <button
      className={`text-xl text-gray-900 select-none hover:opacity-50 ${
        vote === -1
          ? "text-red-500 hover:text-red-900 disabled:opacity-100"
          : "hover:text-red-500 opacity-20"
      }`}
      onClick={() => setVote(-1)}
      title={`Dislike this voice`}
      disabled={vote === -1}
    >
      <MUIIcon icon={ThumbsDownIcon} alt="dislike" />
    </button>
  </div>
);
