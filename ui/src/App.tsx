import { Routes, Route } from "react-router-dom";
import Home from "./pages/Home.tsx";
import Setup from "./pages/Setup.tsx";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/setup" element={<Setup />} />
    </Routes>
  );
}
