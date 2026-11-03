// TypeScript 7 requires resolvable types for side-effect imports of
// non-code assets. Stylesheets stay bundled by Vite, so a single
// ambient declaration covers every ".css" import in apps and packages.
declare module "*.css";
