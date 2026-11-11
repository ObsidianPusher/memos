import { JSDOM } from "jsdom";

// Node 26's built-in storage can shadow jsdom's storage in Vitest 3.
const storage = new JSDOM("", { url: "http://localhost" }).window.localStorage;
Object.defineProperty(globalThis, "localStorage", { value: storage, configurable: true });
Object.defineProperty(window, "localStorage", { value: storage, configurable: true });
