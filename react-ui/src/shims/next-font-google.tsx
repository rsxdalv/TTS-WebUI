/**
 * A `next/font/google` compatible shim on top of @fontsource/inter.
 *
 * Next downloads the font at build time and gives back a class name that
 * applies it. This shim uses the font files from the package instead, so
 * the app works without a network connection at build time.
 */
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "./inter.css";

export interface NextFontResult {
  className: string;
  style: { fontFamily: string };
  variable: string;
}

export const Inter = (_options?: {
  subsets?: string[];
  weight?: string | string[];
  style?: string | string[];
}): NextFontResult => ({
  className: "font-inter",
  style: { fontFamily: "'Inter', sans-serif" },
  variable: "--font-inter",
});

export default { Inter };
