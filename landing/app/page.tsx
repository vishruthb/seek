import dynamic from "next/dynamic";

import Footer from "@/components/Footer";
import GuideRail from "@/components/GuideRail";
import Hero from "@/components/Hero";
import TerminalDemo from "@/components/TerminalDemo";

const InstallSection = dynamic(() => import("@/components/InstallSection"));

const centeredSectionClass =
  "flex min-h-screen w-full snap-start items-center justify-center px-4 py-8 sm:px-8 lg:px-10 lg:py-10 xl:px-14 2xl:px-16 2xl:py-14";
const flowSectionClass =
  "flex min-h-screen w-full snap-start items-start justify-center px-4 py-10 sm:px-8 lg:px-10 lg:py-12 xl:px-14 2xl:px-16 2xl:py-16";

export default function Page() {
  return (
    <main className="terminal-grid h-screen snap-y snap-mandatory overflow-x-hidden overflow-y-auto">
      <section className={centeredSectionClass}>
        <div className="w-full max-w-[1680px]">
          <div className="section-shell flex w-full items-center overflow-visible rounded-[2.2rem] px-5 py-6 sm:px-8 sm:py-8 lg:min-h-[min(79vh,890px)] lg:overflow-hidden lg:px-10 lg:py-9 xl:px-12 2xl:min-h-[min(77vh,940px)] 2xl:px-14 2xl:py-10">
            <div className="grid w-full gap-8 lg:flex-1 lg:grid-cols-[minmax(360px,0.85fr)_minmax(580px,1.15fr)] lg:items-stretch xl:gap-12 2xl:grid-cols-[minmax(420px,0.9fr)_minmax(660px,1.1fr)] 2xl:gap-16">
              <Hero />
              <div className="hidden lg:flex lg:min-h-[36rem] lg:flex-col lg:justify-end 2xl:min-h-[39rem]">
                <div className="w-full max-w-[64rem] self-center">
                  <TerminalDemo compact />
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
      <section className={flowSectionClass}>
        <div className="w-full max-w-[1680px]">
          <GuideRail />
        </div>
      </section>
      <section className={centeredSectionClass}>
        <div className="flex w-full max-w-[1680px] flex-col gap-8">
          <InstallSection />
          <Footer />
        </div>
      </section>
    </main>
  );
}
