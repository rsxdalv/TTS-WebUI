import React from "react";

export const MUIIcon = ({
  icon,
  alt,
  className = "",
}: {
  icon: { src: string; width: number; height: number };
  alt: string;
  className?: string;
}) => (
  <img
    src={icon.src}
    width={icon.width}
    height={icon.height}
    alt={alt}
    className={`select-none ${className}`}
  />
);
