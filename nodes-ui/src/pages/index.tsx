import dynamic from 'next/dynamic';
import Head from 'next/head';

// Dynamically import the editor workspace to avoid SSR issues with Rete.js
const EditorWorkspace = dynamic(
  () => import('@/components/EditorWorkspace').then(mod => mod.EditorWorkspace),
  { 
    ssr: false,
    loading: () => (
      <div className="flex items-center justify-center h-screen bg-background">
        <div className="text-muted-foreground">Loading editor...</div>
      </div>
    ),
  }
);

export default function Home() {
  return (
    <>
      <Head>
        <title>TTS-WebUI Nodes</title>
        <meta name="description" content="Graph-based node editor for TTS and audio processing" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <link rel="icon" href="/favicon.ico" />
      </Head>
      <main className="h-screen w-screen overflow-hidden">
        <EditorWorkspace />
      </main>
    </>
  );
}
