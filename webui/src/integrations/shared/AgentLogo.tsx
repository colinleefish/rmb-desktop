export function AgentLogo({
  logo,
  inactive = false,
  size = 16,
}: {
  logo: string;
  inactive?: boolean;
  size?: number;
}) {
  const box = { width: size, height: size, minWidth: size, minHeight: size };

  return (
    <span
      className={[
        "inline-flex shrink-0 items-center justify-center overflow-hidden",
        inactive ? "opacity-35 grayscale" : "",
      ].join(" ")}
      style={box}
      aria-hidden
    >
      <img
        src={logo}
        alt=""
        width={size}
        height={size}
        className="block object-contain"
        style={box}
        draggable={false}
      />
    </span>
  );
}
