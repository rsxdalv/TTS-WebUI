export interface VoteCount {
  [option: string]: number;
}

// const BASE_URL = "http://localhost:3001";

const callAPI = async (
  url: string,
  method: string = "GET",
  body: any = null
): Promise<any> => {
  try {
    const options: RequestInit = {
      method,
    };

    if (body) {
      options.body = JSON.stringify(body);
      options.headers = { "Content-Type": "application/json" };
    }

    const response = await fetch(url, options);

    if (response.ok) {
      return await response.json();
    } else {
      throw new Error(`API request failed with status ${response.status}`);
    }
  } catch (error) {
    throw new Error(`An error occurred: ${error}`);
  }
};

export const submitVote = async (
  option: string,
  value: number
): Promise<void> => {
  // const url = `${BASE_URL}/vote/${encodeURIComponent(option)}/${value}`;
  // try {
  //   await callAPI(url, "POST");
  //   console.log("Vote counted successfully.");
  // } catch (error) {
  //   console.error("Failed to submit vote:", error);
  // }
};

export const getResults = async () => {
  // const url = `${BASE_URL}/results`;
  // try {
  //   const results: VoteCount = await callAPI(url);
  //   return results;
  // } catch (error) {
  //   console.error("Failed to get vote results:", error);
  //   throw error;
  // }
  return {};
};

// Usage example
// submitVote("option1", 1); // Submit a vote for option1 with a value of 1
// submitVote("option2", -1); // Submit a vote for option2 with a value of -1
// getResults(); // Fetch and display the current vote results
