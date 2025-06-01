import { Inter } from "next/font/google";
import React from "react";
import Head from "next/head";
import { FavoritesProvider } from "./FavoritesProvider";
import { LocalVotesProvider, VotesProvider } from "@/components/VotesProvider";
import { Header } from "./Header";
export const inter = Inter({ subsets: ["latin"] });

export const Template = ({
  children,
  title,
}: {
  children: React.ReactNode;
  title?: string;
}) => (
  <FavoritesProvider>
    <LocalVotesProvider>
      <VotesProvider>
        <main
          className={`flex min-h-screen flex-col items-center justify-start px-8 py-6 gap-y-2 ${inter.className}`}
        >
          <Header />
          {title && (
            <Head>
              <title>{`${title} - TTS WebUI`}</title>
            </Head>
          )}
          {children}
        </main>
      </VotesProvider>
    </LocalVotesProvider>
  </FavoritesProvider>
);
