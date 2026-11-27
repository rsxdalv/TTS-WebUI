import { Template } from "../components/Template";

export default function History() {
  return (
    <Template title="History">
      <div>
        <h1>History</h1>
        <p>
          Generated outputs can be found in the <code>outputs</code> directory.
        </p>
        <p>
          You can view the generations through the History and Favorites tabs
          above.
        </p>
      </div>
    </Template>
  );
}
