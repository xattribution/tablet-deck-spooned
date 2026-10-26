import "./styles/themes.css";
import "./styles/app.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { start } from "./lib/net";

// iPadOS pinch zoom ignores user-scalable=no; stop it here. (Double-tap zoom is off
// through touch-action: manipulation in the CSS.)
document.addEventListener("gesturestart", (e) => e.preventDefault(), { passive: false });

mount(App, { target: document.getElementById("app")! });
start();

// The app works offline for its own files; the PC is still needed to press anything.
if ("serviceWorker" in navigator) navigator.serviceWorker.register("/sw-app.js").catch(() => {});
