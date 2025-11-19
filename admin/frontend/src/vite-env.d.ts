/**
 * Vite Env.D
 *
 * Part of MantisDB - High-performance multi-model database.
 * See CONTRIBUTING.md for code standards and comment guidelines.
 */

/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly DEV: boolean;
  readonly PROD: boolean;
  readonly MODE: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
