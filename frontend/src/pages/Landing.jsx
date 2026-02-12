import HeroSection from '../components/HeroSection';
import StatsBar from '../components/landing/StatsBar';
import PipelineDemo from '../components/landing/PipelineDemo';
import FeaturesGrid from '../components/landing/FeaturesGrid';
import ComparisonTable from '../components/landing/ComparisonTable';
import TechStackBadges from '../components/landing/TechStackBadges';
import LandingFooter from '../components/landing/LandingFooter';

export default function Landing() {
  return (
    <div className="flex flex-col items-center w-full">
      <HeroSection />
      <StatsBar />
      <PipelineDemo />
      <FeaturesGrid />
      <ComparisonTable />
      <TechStackBadges />
      <LandingFooter />
    </div>
  );
}
