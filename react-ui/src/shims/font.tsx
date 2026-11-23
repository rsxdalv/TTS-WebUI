/**
 * The `Inter` font on top of @fontsource/inter.
 *
 * It returns a class name that applies the font. The font files come from
 * the package, so the app works without a network connection at build time.
 */
import "@fontsource/inter/400.css";
import "@fontsource/inter/500.css";
import "@fontsource/inter/600.css";
import "@fontsource/inter/700.css";
import "./inter.css";

export interface FontResult {
  className: string;
  style: { fontFamily: string };
  variable: string;
}

export const Inter = (_options?: {
  subsets?: string[];
  weight?: string | string[];
  style?: string | string[];
}): FontResult => ({
  className: "font-inter",
  style: { fontFamily: "'Inter', sans-serif" },
  variable: "--font-inter",
});

export default { Inter };
