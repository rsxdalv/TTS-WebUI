import { useEffect, useState } from "react";
import React from "react";
import { CardBig } from "./CardBig";
import { Voice } from "../types/Voice";

const FavoritesContext = React.createContext<{
  favorites: string[];
  setFavorites: (favorites: string[]) => void;
}>({
  favorites: [],
  setFavorites: () => {},
});

// Refactor Favorites provider
export const FavoritesProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [favorites, setFavoritesState] = useState<string[]>([]);
  useEffect(() => {
    if (typeof window !== "undefined") {
      setFavoritesState(JSON.parse(localStorage.getItem("favorites") || "[]"));
    }
  }, []);
  const setFavorites = (favorites: string[]) => {
    setFavoritesState(favorites);
    localStorage.setItem("favorites", JSON.stringify(favorites));
  };
  return (
    <FavoritesContext.Provider value={{ favorites, setFavorites }}>
      {children}
    </FavoritesContext.Provider>
  );
};

export const useFavorites = () => {
  const { favorites, setFavorites } = React.useContext(FavoritesContext);
  return [favorites, setFavorites] as const;
};

export const Favorites = ({ voices }: { voices: Voice[] }) => {
  const [favorites] = useFavorites();
  const favoritesHydrated = voices.filter((voice) =>
    favorites.includes(voice.download)
  );
  return (
    <details className="w-full duration-300 transition-all ease-in-out">
      <summary className="bg-gray-600 open:bg-gray-500 px-5 py-3 text-lg cursor-pointer text-center rounded-lg">
        Favorite voices
      </summary>
      <div className="mt-2 px-5 py-3 bg-gray-800 rounded-lg text-sm font-light">
        <div className="grid gap-4 grid-cols-1 md:grid-cols-2 lg:grid-cols-3">
          {favoritesHydrated.map((voice) => (
            <CardBig key={voice.download} voice={voice} />
          ))}
        </div>
      </div>
    </details>
  );
};

export const saveOrDeleteFromFavorites = (
  favorites: string[],
  isFavorite: boolean,
  download: string,
  setFavorites: (favorites: string[]) => void
) => {
  const index = favorites.indexOf(download);
  const favoritesCopy = [...favorites];
  if (isFavorite) {
    if (index > -1) {
      favoritesCopy.splice(index, 1);
    }
  } else {
    favoritesCopy.push(download);
  }
  setFavorites(favoritesCopy);
  console.log(favorites);
};
