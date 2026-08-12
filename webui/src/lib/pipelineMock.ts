/** Dev-only: pipeline health page uses mock data instead of rmbd. */
export function isPipelineMocked(): boolean {
  if (import.meta.env.VITE_MOCK_PIPELINE === "true") return true;
  try {
    return localStorage.getItem("rmb.mockPipeline") === "1";
  } catch {
    return false;
  }
}
