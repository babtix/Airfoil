import React from 'react';

export const AirfoilSVG: React.FC<{ className?: string }> = ({ className = 'w-full' }) => {
  return (
    <svg
      viewBox="0 0 360 170"
      className={className}
      style={{ width: '100%', height: 'auto', display: 'block' }}
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        {/* Subtle glow for lift vector */}
        <filter id="lift-glow" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="1.5" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>

        {/* Gradient for aerodynamic wing fill */}
        <linearGradient id="wing-grad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#1c1c1c" />
          <stop offset="50%" stopColor="#141414" />
          <stop offset="100%" stopColor="#0d0d0d" />
        </linearGradient>
      </defs>

      {/* ── Background Calibration Grid & Streamlines ── */}
      {/* Upper Laminar Streamlines (Suction Side) */}
      {[22, 34, 46].map((y, i) => (
        <path
          key={`upper-stream-${y}`}
          d={`M 15 ${y + 4} C 90 ${y - 4} 135 ${y - 8} 185 ${y - 6} C 250 ${y} 305 ${y + 6} 345 ${y + 8}`}
          stroke="var(--line)"
          strokeWidth={i === 2 ? "1.2" : "0.9"}
          fill="none"
          strokeDasharray={i === 1 ? "4 3" : undefined}
          opacity={0.85}
        />
      ))}

      {/* Lower Streamlines (Pressure Side) */}
      {[124, 138, 150].map((y) => (
        <path
          key={`lower-stream-${y}`}
          d={`M 15 ${y - 4} C 95 ${y + 2} 175 ${y + 4} 260 ${y + 2} C 300 ${y} 330 ${y - 2} 345 ${y - 2}`}
          stroke="var(--line)"
          strokeWidth="0.9"
          fill="none"
          opacity={0.7}
        />
      ))}

      {/* ── Aerodynamic Airfoil Wing Profile (NACA-Inspired) ── */}
      <path
        d="M 48 95 
           C 48 82, 65 58, 120 54 
           C 180 50, 240 76, 282 95 
           C 235 106, 175 108, 120 106 
           C 75 104, 48 102, 48 95 Z"
        fill="url(#wing-grad)"
        stroke="var(--prim)"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />

      {/* ── Chord Line (Straight Reference Line connecting LE to TE) ── */}
      <line
        x1="48"
        y1="95"
        x2="282"
        y2="95"
        stroke="#666666"
        strokeWidth="0.9"
        strokeDasharray="4 3"
      />

      {/* ── Mean Camber Line (Curved Mean Axis between Upper & Lower Surfaces) ── */}
      <path
        d="M 48 95 
           C 75 75, 125 72, 170 75 
           C 220 78, 258 89, 282 95"
        stroke="var(--kai)"
        strokeWidth="1.2"
        strokeDasharray="4 3"
        fill="none"
      />

      {/* ── Lift Vector (Aerodynamic Lift = Signal Score) ── */}
      <g filter="url(#lift-glow)">
        {/* Vector Line */}
        <line
          x1="138"
          y1="54"
          x2="138"
          y2="15"
          stroke="var(--kai)"
          strokeWidth="1.6"
        />
        {/* Arrow Head */}
        <path
          d="M 133 22 L 138 12 L 143 22"
          stroke="var(--kai)"
          strokeWidth="1.6"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
        {/* Center of Pressure Node */}
        <circle cx="138" cy="54" r="2.5" fill="var(--kai)" />
      </g>

      {/* ── Telemetry Annotations & Precision Labels ── */}

      {/* Lift = Score Label */}
      <text
        x="148"
        y="21"
        fontFamily="JetBrains Mono, monospace"
        fontSize="9"
        fontWeight="600"
        fill="var(--kai)"
        letterSpacing="0.08em"
      >
        LIFT = SCORE
      </text>

      {/* Camber Label (Cleanly placed above the red dashed camber curve) */}
      <text
        x="108"
        y="68"
        fontFamily="JetBrains Mono, monospace"
        fontSize="7.5"
        fontWeight="500"
        fill="var(--kai)"
        letterSpacing="0.06em"
      >
        CAMBER
      </text>

      {/* Chord Line Label (Cleanly placed along the gray straight chord line) */}
      <text
        x="192"
        y="92"
        fontFamily="JetBrains Mono, monospace"
        fontSize="7.5"
        fontWeight="500"
        fill="#888888"
        letterSpacing="0.06em"
      >
        CHORD
      </text>

      {/* Leading Edge Label & Pointer */}
      <circle cx="48" cy="95" r="2" fill="var(--prim)" />
      <line x1="48" y1="99" x2="48" y2="114" stroke="var(--line)" strokeWidth="0.8" />
      <text
        x="22"
        y="125"
        fontFamily="JetBrains Mono, monospace"
        fontSize="7.5"
        fontWeight="400"
        fill="var(--sec)"
        letterSpacing="0.06em"
      >
        LEADING EDGE
      </text>

      {/* Trailing Edge Label & Pointer */}
      <circle cx="282" cy="95" r="2" fill="var(--prim)" />
      <line x1="282" y1="99" x2="282" y2="114" stroke="var(--line)" strokeWidth="0.8" />
      <text
        x="248"
        y="125"
        fontFamily="JetBrains Mono, monospace"
        fontSize="7.5"
        fontWeight="400"
        fill="var(--sec)"
        letterSpacing="0.06em"
      >
        TRAILING EDGE
      </text>
    </svg>
  );
};
