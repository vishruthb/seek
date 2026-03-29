import dynamic from "next/dynamic";

import Footer from "@/components/Footer";
import GuideRail from "@/components/GuideRail";
import Hero from "@/components/Hero";
import TerminalDemo from "@/components/TerminalDemo";

const InstallSection = dynamic(() => import("@/components/InstallSection"));

const centeredSectionClass =
  "flex min-h-screen w-full snap-start items-center px-4 py-8 sm:px-8 lg:px-10 xl:px-14";
const flowSectionClass =
  "flex min-h-screen w-full snap-start items-start px-4 py-10 sm:px-8 lg:px-10 xl:px-14";

export default function Page() {
  return (
    <main className="terminal-grid h-screen snap-y snap-mandatory overflow-x-hidden overflow-y-auto">
      <section className={centeredSectionClass}>
        <div className="section-shell w-full overflow-visible rounded-[2.2rem] px-5 py-6 sm:px-8 sm:py-8 lg:min-h-[82vh] lg:overflow-hidden lg:px-10 lg:py-10 xl:px-12">
          <div className="grid w-full gap-8 lg:grid-cols-[minmax(360px,0.8fr)_minmax(620px,1.2fr)] lg:items-center xl:gap-12">
            <Hero />
            <div className="hidden lg:flex">
              <TerminalDemo compact />
            </div>
          </div>
        </div>
      </section>
      <section className={flowSectionClass}>
        <GuideRail />
      </section>
      <section className={centeredSectionClass}>
        <div className="flex w-full flex-col gap-8">
          <InstallSection />
          <Footer />
        </div>
      </section>
    </main>
  );
}
