import { sql } from "../lib/sql";

export default function DB({ generations = [] }: { generations?: any[] }) {
  // generations = await sql`
  // SELECT (1, 2, 3)
  //   `;

  return (
    <div>
      <h1>DB</h1>
      <p>DB for working with the generated audio and managing the models.</p>
      <pre>{JSON.stringify(generations, null, 2)}</pre>
    </div>
  );
}

export const getStaticProps = async () => {
  //   const generations = [];
  // SELECT * FROM generations
  return {
    props: {
      generations: [],
    },
  };
  try {
    const generations = await sql`
    SELECT * FROM generations
      `;
    return {
      props: {
        generations: generations,
      },
    };
  } catch (error) {
    console.error(error);
    return {
      props: {
        generations: [],
      },
    };
  }
  const generations = await sql`
  SELECT (1, 2, 3)
    `;
  return {
    props: {
      generations: generations,
    },
  };
};
