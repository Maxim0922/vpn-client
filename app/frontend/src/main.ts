import "../../../design/bundle.css";
import "./tokens.css";
import { mount } from "svelte";
import App from "./App.svelte";
import { initStores } from "./stores";
import { initTheme } from "./theme";

initTheme();
initStores();

export default mount(App, { target: document.getElementById("app")! });
