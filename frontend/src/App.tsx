import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router-dom";
import "bootstrap/dist/js/bootstrap.bundle.min.js";
import "bootswatch/dist/slate/bootstrap.min.css";
import "./App.css";
import Navbar from "./components/Navbar";

const Home = lazy(() => import("./pages/Home"));
const InteractiveShell = lazy(() => import("./pages/InteractiveShell"));
const ApiDocs = lazy(() => import("./pages/ApiDocs"));

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
      <Suspense fallback={<div className="container mt-4">Loading...</div>}>
      <Routes>
        <Route path="/" element={<Home />} errorElement={<InternalError />} />
        <Route
          path="/shell/:uuid"
          element={<InteractiveShell />}
          errorElement={<InternalError />}
        />
        <Route path="/docs" element={<ApiDocs />} errorElement={<InternalError />} />
        <Route
          path="*"
          element={<NotFound />}
          errorElement={<InternalError />}
        />
      </Routes>
      </Suspense>
    </>
  );
}
