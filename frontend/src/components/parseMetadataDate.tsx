export const parseMetadataDate = (date: string) => {
  // detect if format is like 2024-08-30 16:19:51.647890
  if (date.includes(" ")) {
    return new Date(date);
  }
  // detect if format is like 2023-05-27_13-06-13
  if (/([0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}-[0-9]{2}-[0-9]{2})/.test(date)) {
    const [year, month, day, hour, minute, second] = date
      .split(/[-_]/)
      .map((x) => parseInt(x));
    return new Date(year, month - 1, day, hour, minute, second);
  }
  return new Date(0);
};
