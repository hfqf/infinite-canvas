import { Suspense } from "react";

import { WorkbenchShell } from "@/features/workbench/components/workbench-shell";

export default function WorkbenchPage() {
    return (
        <Suspense>
            <WorkbenchShell />
        </Suspense>
    );
}
