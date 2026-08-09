/** Dev-only: integration settings use mock data instead of reading/writing real agent configs. */
export function isSetupMocked(): boolean {
  if (import.meta.env.VITE_MOCK_SETUP === "true") return true;
  if (!import.meta.env.DEV) return false;
  try {
    return localStorage.getItem("rmb.mockSetup") === "1";
  } catch {
    return false;
  }
}
