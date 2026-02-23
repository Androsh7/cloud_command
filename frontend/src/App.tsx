import { Route, Routes } from "react-router-dom";
import Home from "./pages/Home";
import "bootstrap/dist/js/bootstrap.bundle.min.js";
import "bootswatch/dist/slate/bootstrap.min.css";
import Navbar from "./components/Navbar";
import CreateAgent from "./pages/CreateAgent";
import DeleteAgent from "./pages/DeleteAgent";
import InteractiveShell from "./pages/InteractiveShell";
import Login from "./pages/Login";

function NotFound() {
  return <div className="container mt-4">Page not found</div>;
}

function InternalError() {
  return <div className="container mt-4">Internal Server Error</div>;
}

export default function App() {
  return (
    <>
      <Navbar />
      <Routes>
        <Route path="/" element={<Home />} errorElement={<InternalError />} />
        <Route
          path="/agents"
          element={<Home />}
          errorElement={<InternalError />}
        />
        <Route
          path="/create_agent"
          element={<CreateAgent />}
          errorElement={<InternalError />}
        />
        <Route
          path="/delete_agent"
          element={<DeleteAgent />}
          errorElement={<InternalError />}
        />
        <Route
          path="/interactive_shell"
          element={<InteractiveShell />}
          errorElement={<InternalError />}
        />
        <Route
          path="/login"
          element={<Login />}
          errorElement={<InternalError />}
        />
        <Route
          path="*"
          element={<NotFound />}
          errorElement={<InternalError />}
        />
      </Routes>
    </>
  );
}
