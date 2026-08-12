export default function Home() {
  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
      <h1 className="text-4xl font-bold tracking-tight">Cascade</h1>
      <p className="max-w-xl text-base text-gray-500 dark:text-gray-400">
        Real-time feed &amp; ranking system — consumer serving infrastructure demo.
      </p>
      <p className="max-w-xl text-sm text-gray-400 dark:text-gray-500">
        This frontend is scaffolded in Phase 0. The home feed, composer, follow graph explorer,
        and live admin metrics dashboard described in{" "}
        <code className="rounded bg-gray-100 px-1 py-0.5 dark:bg-gray-800">
          IMPLEMENTATION_PLAN.md
        </code>{" "}
        §10 are built starting in Phase 10, once the API Gateway (Phase 9) has real endpoints to
        call.
      </p>
    </main>
  );
}
