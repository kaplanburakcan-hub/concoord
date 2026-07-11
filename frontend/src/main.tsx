import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./index.css";
import { initOfflineSync } from "./offline/queue";

// Faz 6 — PWA: kabuk cache'i için service worker (yalnızca prod build'de;
// geliştirmede Vite HMR ile çakışmasın) + çevrimdışı kuyruk senkronu.
if ("serviceWorker" in navigator && import.meta.env.PROD) {
  window.addEventListener("load", () => {
    navigator.serviceWorker
      .register("/sw.js")
      .then((reg) => {
        // Faz 10 — yeni sürüm hazır olduğunda bekleyen SW'yi anında devral.
        reg.addEventListener("updatefound", () => {
          const sw = reg.installing;
          if (!sw) return;
          sw.addEventListener("statechange", () => {
            if (sw.state === "installed" && navigator.serviceWorker.controller) {
              sw.postMessage("SKIP_WAITING");
            }
          });
        });
      })
      .catch(() => {
        /* SW kaydolamazsa uygulama normal (yalnızca çevrimiçi) çalışır */
      });
  });
}
initOfflineSync();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
