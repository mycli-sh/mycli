export function Logo({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 340 70"
      className={className}
      role="img"
      aria-label="mycli"
    >
      <defs>
        <linearGradient id="logo-chevron" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#8B5CF6" />
          <stop offset="100%" stopColor="#A78BFA" />
        </linearGradient>
      </defs>
      <text
        x="0"
        y="55"
        fill="url(#logo-chevron)"
        fontFamily="monospace"
        fontSize="56"
        fontWeight="700"
      >
        &gt;
      </text>
      <text x="38" y="55" fontFamily="monospace" fontSize="56" fontWeight="700">
        <tspan fill="#ffffff">my</tspan>
        <tspan fill="#8B5CF6">cli</tspan>
      </text>
    </svg>
  );
}
