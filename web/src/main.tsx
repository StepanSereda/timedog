import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route, NavLink } from "react-router-dom";
import { ScanPage } from "./ScanPage";
import { ReportPage } from "./ReportPage";
import "./App.css";

function Layout({ children }: { children: React.ReactNode }) {
  return (
    <div className="app">
      <header className="topnav">
        <NavLink to="/" end className={({ isActive }) => (isActive ? "active" : "")}>
          Новый скан
        </NavLink>
        <NavLink to="/report" className={({ isActive }) => (isActive ? "active" : "")}>
          Открыть отчёт
        </NavLink>
      </header>
      <main>{children}</main>
    </div>
  );
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<ScanPage />} />
          <Route path="/report" element={<ReportPage />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  </React.StrictMode>
);
