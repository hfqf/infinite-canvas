export function SiteFooter() {
    return (
        <footer className="border-t border-slate-200 bg-white px-4 py-8 sm:px-6 lg:px-8">
            <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 text-sm text-slate-500 md:flex-row">
                <div className="flex items-center gap-2">
                    <img src="/haotushow-logo.png" alt="好图秀" className="size-6 rounded object-contain" />
                    <span>© 2026 好图秀AI haotushow.com</span>
                </div>
                <div className="flex flex-wrap items-center justify-center gap-3">
                    <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" className="transition hover:text-slate-800">
                        苏ICP备18027098号-7
                    </a>
                    <a href="https://beian.mps.gov.cn/#/query/webSearch?code=32011502013847" target="_blank" rel="noreferrer" className="transition hover:text-slate-800">
                        苏公网安备32011502013847号
                    </a>
                </div>
            </div>
        </footer>
    );
}
