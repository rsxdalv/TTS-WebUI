
export type GradioFile = {
  name: string;
  data: string;
  url: string; // appears to be the new 'data'
  size?: number;
  is_file?: boolean;
  orig_name?: string;
};
