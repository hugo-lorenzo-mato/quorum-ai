const badges = [
  { label: 'React', color: 'bg-[#61dafb]/10 text-[#61dafb]' },
  { label: 'Vite', color: 'bg-[#646cff]/10 text-[#646cff]' },
  { label: 'Tailwind CSS', color: 'bg-[#38bdf8]/10 text-[#38bdf8]' },
  { label: 'Spring Boot', color: 'bg-[#6db33f]/10 text-[#6db33f]' },
  { label: 'WebFlux', color: 'bg-[#6db33f]/10 text-[#6db33f]' },
  { label: 'SSE', color: 'bg-[#f97316]/10 text-[#f97316]' },
  { label: 'Zustand', color: 'bg-[#443e38]/10 text-[#d4a574]' },
  { label: 'Lucide Icons', color: 'bg-[#f56565]/10 text-[#f56565]' },
];

export default function TechStackBadges() {
  return (
    <section className="w-full max-w-3xl mx-auto px-6 py-12 text-center">
      <h2 className="text-lg font-semibold text-muted-foreground mb-6">
        Built with
      </h2>
      <div className="flex flex-wrap items-center justify-center gap-2">
        {badges.map((badge) => (
          <span
            key={badge.label}
            className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-medium ${badge.color} ring-1 ring-current/10`}
          >
            {badge.label}
          </span>
        ))}
      </div>
    </section>
  );
}
