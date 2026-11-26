import React from "react";
import { Button } from "./ui/button";
import { TrashIcon } from "lucide-react";
import { errorText, goGetJson, goSendJson } from "../functions/goData";

type HuggingFaceCache = {
  headers: string[];
  rows: Object[];
  warnings?: string[];
  cache_dir?: string;
  exists?: boolean;
};

type DeleteAnswer = {
  deleted: unknown[];
  not_found: unknown[];
  errors: string[];
};

export const ExtensionHuggingFaceCacheManager = ({}) => {
  const [huggingFaceCache, setHuggingFaceCache] =
    React.useState<HuggingFaceCache>({
      headers: [],
      rows: [],
    });
  const [loading, setLoading] = React.useState<boolean>(false);
  const [sortKey, setSortKey] = React.useState<string>("repo_id");
  const [sortDirection, setSortDirection] = React.useState<string>("asc");
  const [error, setError] = React.useState<string>("");

  // The Go core scans the cache itself (no Gradio host needed).
  const fetchHuggingFaceCache = async () => {
    setLoading(true);
    try {
      const result = await goGetJson<HuggingFaceCache>(
        "/api/go-data/hf-cache/scan"
      );
      setHuggingFaceCache(result);
    } catch (e) {
      setError(errorText(e));
    }
    setLoading(false);
  };

  React.useEffect(() => {
    fetchHuggingFaceCache();
  }, []);

  if (loading && !huggingFaceCache.headers.length) {
    return <div>Loading HuggingFace Cache...</div>;
  }

  const { rows } = huggingFaceCache;

  const finalHeaderSelection = [
    "repo_id",
    "repo_type",
    "refs",
    "size_on_disk_str",
    "commit_hash",
    // "repo_size_on_disk",
    // "revision_size_on_disk",
    // "nb_files",
    "last_accessed_str",
    "last_modified_str",
    // "last_modified",
    // "snapshot_path",

    "delete",
  ];

  const deleteRevision = async (row: any) => {
    setError("");
    try {
      const answer = await goSendJson<DeleteAnswer>(
        "POST",
        "/api/go-data/hf-cache/delete",
        {
          revisions: [
            {
              commit_hash: row["commit_hash"],
              repo_id: row["repo_id"],
              repo_type: row["repo_type"],
            },
          ],
        }
      );
      if (answer.errors?.length) setError(answer.errors.join("\n"));
    } catch (e) {
      setError(errorText(e));
    }
    fetchHuggingFaceCache();
  };

  const cellRenderer = (header: string, row: any) => {
    const cell = row[header];
    if (header === "commit_hash") {
      return <span title={cell}>{cell.substring(0, 8) + "..."}</span>;
    }
    if (header === "repo_id") {
      return (
        <a
          href={`https://huggingface.co/${cell}`}
          target="_blank"
          rel="noreferrer"
          className="hover:underline"
        >
          {cell}
        </a>
      );
    }
    if (header === "delete") {
      return (
        <Button variant="destructive" onClick={() => deleteRevision(row)}>
          Delete
          <TrashIcon className="ml-2 h-5 w-5 flex-shrink-0" />
        </Button>
      );
    }
    return cell;
  };

  const sortRows = (header: string, rows: any[], sortDirection: string) => {
    if (header === "delete") {
      return rows;
    }
    const getValue = (row: any) => {
      if (header === "size_on_disk_str") {
        return row["revision_size_on_disk"];
      }
      if (header === "last_accessed_str") {
        return row["last_accessed"];
      }
      if (header === "last_modified_str") {
        return row["last_modified"];
      }
      if (typeof row[header] === "string") {
        return row[header].toLowerCase();
      }
      return row[header];
    };
    return rows.sort((a, b) => {
      const aValue = getValue(a);
      const bValue = getValue(b);
      if (sortDirection === "asc") {
        if (aValue < bValue) {
          return -1;
        }
        if (aValue > bValue) {
          return 1;
        }
        return 0;
      } else {
        if (aValue > bValue) {
          return -1;
        }
        if (aValue < bValue) {
          return 1;
        }
        return 0;
      }
    });
  };

  return (
    <div className="flex flex-col gap-2 w-full justify-center">
      <h1 className="text-xl text-gray-900">HuggingFace Cache</h1>
      <p>
        The following table shows all of the 'revisions' that have been cached
        by HuggingFace for all of the projects that you have used in the past.
        You can delete any of these revisions by clicking the 'Delete' button.
      </p>

      <p>
        Note that if the files are still being used by another revision, they
        will not be deleted. <br />
        Files will only be deleted once no revision is using them. <br />
        (For example, when all of the 'facebook/musicgen-small' revisions are
        deleted, the files will be deleted.)
      </p>
      {error && (
        <pre className="text-red-600 whitespace-pre-wrap" role="alert">
          {error}
        </pre>
      )}
      {huggingFaceCache.exists === false && (
        <p>No cache folder yet: {huggingFaceCache.cache_dir}</p>
      )}
      {!!huggingFaceCache.warnings?.length && huggingFaceCache.exists && (
        <details>
          <summary>
            {huggingFaceCache.warnings.length} warning(s) while scanning
          </summary>
          <pre className="whitespace-pre-wrap text-xs">
            {huggingFaceCache.warnings.join("\n")}
          </pre>
        </details>
      )}
      <div className="overflow-x-scroll w-full">
        <div className="align-middle inline-block w-full shadow overflow-hidden sm:rounded-lg border-b border-gray-200">
          <table className="w-full">
            <thead>
              <tr>
                {finalHeaderSelection.map((header) => (
                  <th
                    key={header}
                    className="px-2 py-3 border-b border-gray-200 text-xs font-medium text-gray-500 cursor-pointer"
                    onClick={() => {
                      setSortKey(header);
                      const newSortDirection =
                        header === sortKey
                          ? sortDirection === "asc"
                            ? "desc"
                            : "asc"
                          : "asc";
                      setSortDirection(newSortDirection);
                      setHuggingFaceCache({
                        ...huggingFaceCache,
                        rows: sortRows(header, rows, newSortDirection),
                      });
                    }}
                  >
                    {header.replace(/_/g, " ").replace("str", "")}
                    {header === sortKey && sortDirection === "asc" && " ↓"}
                    {header === sortKey && sortDirection === "desc" && " ↑"}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {rows.map((row) => (
                <tr key={row["commit_hash"]}>
                  {finalHeaderSelection.map((header, index) => (
                    <td
                      key={index}
                      className="px-6 py-4 whitespace-no-wrap text-sm leading-5 font-medium text-gray-900"
                    >
                      {cellRenderer(header, row)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
