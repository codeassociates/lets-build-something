/**
 * Category artwork. Drawn inline rather than loaded as images: the catalog
 * needs to look furnished with no asset pipeline, no external requests, and
 * no broken-image placeholders when a photo has not been uploaded yet.
 */
export function EquipmentIcon({ category, className }: { category: string; className?: string }) {
  const common = {
    fill: 'none',
    stroke: 'currentColor',
    strokeWidth: 1.6,
    strokeLinecap: 'round' as const,
    strokeLinejoin: 'round' as const,
  }
  return (
    <svg viewBox="0 0 48 48" className={className} style={{ color: tint(category) }} aria-hidden>
      {art(category, common)}
    </svg>
  )
}

function tint(category: string): string {
  switch (category) {
    case 'breaking':   return '#8a5a2b'
    case 'concrete':   return '#5f6b76'
    case 'pumps':      return '#2c6b8a'
    case 'painting':   return '#7a5194'
    case 'compaction': return '#7b6a2c'
    case 'power':      return '#a06020'
    case 'access':     return '#2f6b52'
    case 'climate':    return '#a04a44'
    default:           return '#5f6b76'
  }
}

function art(category: string, p: object) {
  switch (category) {
    case 'breaking': // jackhammer
      return (<>
        <rect x="18" y="5" width="12" height="13" rx="2" {...p} />
        <path d="M21 18v6h6v-6" {...p} />
        <path d="M24 24v11" {...p} />
        <path d="M20.5 35h7l-3.5 7z" {...p} />
        <path d="M14 10h4M30 10h4" {...p} />
      </>)
    case 'concrete': // mixer drum
      return (<>
        <ellipse cx="24" cy="20" rx="12" ry="9" transform="rotate(-18 24 20)" {...p} />
        <path d="M16 27l-3 12h22l-3-8" {...p} />
        <circle cx="14" cy="41" r="3" {...p} />
        <circle cx="33" cy="41" r="3" {...p} />
        <path d="M19 15l9 4" {...p} />
      </>)
    case 'pumps':
      return (<>
        <circle cx="21" cy="26" r="9" {...p} />
        <path d="M21 17V9h9" {...p} />
        <path d="M30 26h9v9" {...p} />
        <circle cx="21" cy="26" r="3" {...p} />
        <path d="M12 38h20" {...p} />
      </>)
    case 'painting': // spray gun
      return (<>
        <path d="M12 20h16l6-5v14l-6-5H12z" {...p} />
        <path d="M18 25v10h-5" {...p} />
        <path d="M38 16l4-3M38 24h5M38 31l4 3" {...p} />
      </>)
    case 'compaction': // plate compactor
      return (<>
        <rect x="10" y="32" width="28" height="7" rx="2" {...p} />
        <rect x="17" y="16" width="14" height="12" rx="2" {...p} />
        <path d="M24 28v4" {...p} />
        <path d="M31 20l7-8" {...p} />
        <path d="M10 42h28" {...p} strokeDasharray="3 3" />
      </>)
    case 'power': // generator
      return (<>
        <rect x="8" y="16" width="32" height="18" rx="3" {...p} />
        <circle cx="18" cy="25" r="4" {...p} />
        <path d="M28 21v8M33 21v8" {...p} />
        <path d="M14 16v-4h8v4" {...p} />
        <path d="M12 34v4M36 34v4" {...p} />
      </>)
    case 'access': // scissor lift
      return (<>
        <rect x="10" y="8" width="28" height="5" rx="1.5" {...p} />
        <path d="M14 13l20 12M34 13L14 25M14 25l20 12M34 25L14 37" {...p} />
        <rect x="10" y="37" width="28" height="5" rx="1.5" {...p} />
      </>)
    case 'climate': // heater
      return (<>
        <rect x="9" y="17" width="30" height="16" rx="8" {...p} />
        <circle cx="18" cy="25" r="5" {...p} />
        <path d="M30 21v8M35 21v8" {...p} />
        <path d="M14 37v4M34 37v4" {...p} />
      </>)
    default:
      return (<>
        <rect x="10" y="14" width="28" height="20" rx="3" {...p} />
        <path d="M18 14v-4h12v4" {...p} />
      </>)
  }
}
