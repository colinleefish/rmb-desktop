import { EMBED_DIMENSION_OPTIONS, type EmbedDimension } from "../lib/embedDimensions";
import { SelectMenu } from "./SelectMenu";

export function EmbedDimensionsSelect({
  value,
  onChange,
  id = "embed-dimensions",
  labelId,
}: {
  value: EmbedDimension;
  onChange: (dims: EmbedDimension) => void;
  id?: string;
  labelId?: string;
}) {
  const options = EMBED_DIMENSION_OPTIONS.map((dims) => ({
    value: dims,
    label: String(dims),
  }));

  return (
    <SelectMenu
      id={id}
      labelId={labelId}
      value={value}
      onChange={onChange}
      options={options}
    />
  );
}
