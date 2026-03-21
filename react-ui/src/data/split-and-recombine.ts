// This file is adapted from the tortoise repository
// https://github.com/neonbjb/tortoise-tts/blob/main/tortoise/utils/text.py

/**
 * Split text it into chunks of a desired length trying to keep sentences intact.
 */
export function splitAndRecombineText(
  textIn: string,
  desiredLength = 200,
  maxLength = 300
) {
  // normalize text, remove redundant whitespace and convert non-ascii quotes to ascii
  const text = textIn
    .replace(/\n\n+/g, "\n")
    .replace(/\s+/g, " ")
    .replace(/[“”]/g, '"');

  let inQuote = false;
  let current = "";
  let splitPos = [] as number[];
  let pos = -1;
  const endPos = text.length - 1;

  const seek = (delta) => {
    for (let i = 0; i < Math.abs(delta); i++) {
      if (delta < 0) {
        pos--;
        current = current.slice(0, -1);
      } else {
        pos++;
        current += text[pos];
      }
      if (text[pos] === '"') {
        inQuote = !inQuote;
      }
    }
    return text[pos];
  };

  const peek = (delta) => {
    const p = pos + delta;
    return p < endPos && p >= 0 ? text[p] : "";
  };

  const rv = [] as string[];
  function commit() {
    rv.push(current);
    current = "";
    splitPos = [];
  }

  while (pos < endPos) {
    var c = seek(1);
    // do we need to force a split?
    if (current.length >= maxLength) {
      if (splitPos.length > 0 && current.length > desiredLength / 2) {
        // we have at least one sentence and we are over half the desired length, seek back to the last split
        var d = pos - splitPos[splitPos.length - 1];
        seek(-d);
      } else {
        // no full sentences, seek back until we are not in the middle of a word and split there
        while (
          !["!", "?", ".", "\n", " "].includes(c) &&
          pos > 0 &&
          current.length > desiredLength
        ) {
          c = seek(-1);
        }
      }
      commit();
    }
    // check for sentence boundaries
    else if (
      !inQuote &&
      (["!", "?", "\n"].includes(c) ||
        (c === "." && ["\n", " "].includes(peek(1))))
    ) {
      // Seek forward if we have consecutive boundary markers but still within the max length
      while (
        pos < text.length - 1 &&
        current.length < maxLength &&
        ["!", "?", "."].includes(peek(1))
      ) {
        c = seek(1);
      }
      splitPos.push(pos);
      if (current.length >= desiredLength) {
        commit();
      }
    }
    // treat end of quote as a boundary if its followed by a space or newline
    else if (inQuote && peek(1) == '"' && ["\n", " "].includes(peek(2))) {
      seek(2);
      splitPos.push(pos);
    }
  }
  rv.push(current);

  // clean up, remove lines with only whitespace or punctuation
  return rv
    .map((s) => s.trim())
    .filter((s) => s.length > 0 && !/^[\s\.,;:!?]*$/.test(s));
}

// CLI self-test code removed: referenced Node.js `module` global which is not
// available in browser bundles (Turbopack). Tests now live in separate test files.
