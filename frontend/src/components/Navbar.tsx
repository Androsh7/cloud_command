import { NavLink } from "react-router-dom";

export default function Navbar() {
  return (
    <nav className="navbar navbar-expand-lg navbar-dark bg-dark">
      <div className="container-fluid">
        <NavLink className="navbar-brand fw-bold text-info" to="/">
          Cloud Command
        </NavLink>

        <button
          className="navbar-toggler"
          type="button"
          data-bs-toggle="collapse"
          data-bs-target="#navbarNav"
        >
          <span className="navbar-toggler-icon"></span>
        </button>

        <div className="collapse navbar-collapse" id="navbarNav">
          <ul className="navbar-nav me-auto mb-2 mb-lg-0">
            <li className="nav-item">
              <NavLink
                to="/"
                end
                className={({ isActive }) =>
                  "nav-link" + (isActive ? " active fw-bold text-info" : "")
                }
              >
                Home
              </NavLink>
            </li>
            <li className="nav-item">
              <NavLink
                to="/create_agent"
                className={({ isActive }) =>
                  "nav-link" + (isActive ? " active fw-bold text-info" : "")
                }
              >
                Create Agent
              </NavLink>
            </li>
            <li className="nav-item">
              <NavLink
                to="/delete_agent"
                className={({ isActive }) =>
                  "nav-link" + (isActive ? " active fw-bold text-info" : "")
                }
              >
                Delete Agent
              </NavLink>
            </li>
            <li className="nav-item">
              <NavLink
                to="/login"
                className={({ isActive }) =>
                  "nav-link" + (isActive ? " active fw-bold text-info" : "")
                }
              >
                Login
              </NavLink>
            </li>
          </ul>
        </div>
      </div>
    </nav>
  );
}
