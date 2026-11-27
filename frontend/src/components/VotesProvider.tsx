import { useEffect, useState } from "react";
import React from "react";
import { VoteCount, getResults, submitVote } from "../data/getVotes";

export const LOCAL_VOTES = "localVotes";

const LocalVotesContext = React.createContext<{
  localVotes: Record<string, number>;
  setLocalVotes: (votes: Record<string, number>) => void;
}>({
  localVotes: {},
  setLocalVotes: () => {},
});

export const LocalVotesProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [votes, setVotesState] = useState<Record<string, number>>({});
  useEffect(() => {
    if (typeof window !== "undefined") {
      setVotesState(JSON.parse(localStorage.getItem(LOCAL_VOTES) || "{}"));
    }
  }, []);
  const setVotes = (votes: Record<string, number>) => {
    setVotesState(votes);
    localStorage.setItem(LOCAL_VOTES, JSON.stringify(votes));
  };
  return (
    <LocalVotesContext.Provider
      value={{ localVotes: votes, setLocalVotes: setVotes }}
    >
      {children}
    </LocalVotesContext.Provider>
  );
};

export const useLocalVotes = () => {
  const { localVotes, setLocalVotes } = React.useContext(LocalVotesContext);
  return [localVotes, setLocalVotes] as const;
};

const VotesContext = React.createContext<{
  votes: Record<string, number>;
  setVote: (subkey: string, value: number) => void;
}>({
  votes: {},
  setVote: () => {},
});

export const VotesProvider = ({ children }: { children: React.ReactNode }) => {
  const [votes, setVotesState] = useState<Record<string, number>>({});

  useEffect(() => {
    getResultsFromAPI();
  }, []);

  const getResultsFromAPI = async () => {
    try {
      const results: VoteCount = await getResults();
      setVotesState(results);
    } catch (error) {
      console.error("Failed to get vote results:", error);
    }
  };

  const updateVotesOnAPI = async (subkey: string, value: number) => {
    try {
      await submitVote(subkey, value);
      setVotesState((votes) => ({
        ...votes,
        [subkey]: (votes[subkey] || 0) + value,
      }));
    } catch (error) {
      console.error("Failed to update votes:", error);
    }
  };

  const setVote = (subkey: string, value: number) => {
    updateVotesOnAPI(subkey, value);
  };

  return (
    <VotesContext.Provider value={{ votes, setVote }}>
      {children}
    </VotesContext.Provider>
  );
};

export const useVotes = () => {
  const { votes, setVote } = React.useContext(VotesContext);
  return [votes, setVote] as const;
};
