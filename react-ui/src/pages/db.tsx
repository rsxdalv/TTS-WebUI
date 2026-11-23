export default function DB() {
  const generations: any[] = [];

  return (
    <div>
      <h1>DB</h1>
      <p>DB for working with the generated audio and managing the models.</p>
      <pre>{JSON.stringify(generations, null, 2)}</pre>
    </div>
  );
}
