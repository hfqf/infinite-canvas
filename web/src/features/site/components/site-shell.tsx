"use client";

import "../styles.css";

import { SiteFooter } from "./site-footer";
import { SiteGallery } from "./site-gallery";
import { SiteHeader } from "./site-header";
import { SiteHero } from "./site-hero";
import { SitePricing } from "./site-pricing";
import { SiteSceneShowcase } from "./site-scene-showcase";
import { SiteStrengths } from "./site-strengths";

export function SiteShell() {
    return (
        <main className="site-page h-full overflow-y-auto bg-[#f6f8fc] text-slate-950">
            <SiteHeader />
            <SiteHero />
            <SiteSceneShowcase />
            <SiteStrengths />
            <SiteGallery />
            <SitePricing />
            <SiteFooter />
        </main>
    );
}
