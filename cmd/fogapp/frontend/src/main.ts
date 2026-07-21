// Fonts are bundled, not fetched. Fog Desktop is a Wails binary that runs
// offline — a Google Fonts CDN link would silently fall back to system sans.
// @fontsource ships the woff2 files through Vite into frontend/dist.
//
// Two faces only: the "Quiet" direction has no display face, because headings
// carried by the UI face at a heavier weight is what keeps it recessive.
import "@fontsource-variable/geist";
import "@fontsource-variable/jetbrains-mono";

import "./app.css";
import App from "./App.svelte";
import { mount } from "svelte";

const app = mount(App, {
    target: document.getElementById("app")!,
});

export default app;
