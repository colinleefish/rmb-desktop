use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;

use crate::bootstrap;

const DEFAULT_BASE_URL: &str = "http://127.0.0.1:19019";

pub struct DaemonManager {
    child: Mutex<Option<Child>>,
    rmbd_path: PathBuf,
}

impl DaemonManager {
    pub fn new() -> Self {
        Self {
            child: Mutex::new(None),
            rmbd_path: find_rmbd_binary(),
        }
    }

    pub fn managed_running(&self) -> bool {
        let mut guard = self.child.lock().unwrap();
        if let Some(child) = guard.as_mut() {
            match child.try_wait() {
                Ok(Some(_)) => {
                    *guard = None;
                    false
                }
                Ok(None) => true,
                Err(_) => false,
            }
        } else {
            false
        }
    }

    pub fn start(&self) -> Result<(), String> {
        if self.managed_running() {
            return Ok(());
        }
        if health_ok(DEFAULT_BASE_URL) {
            return Ok(());
        }

        let mut guard = self.child.lock().unwrap();
        if guard.is_some() {
            return Ok(());
        }

        let child = Command::new(&self.rmbd_path)
            .arg("serve")
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn()
            .map_err(|e| format!("failed to start rmbd at {}: {e}", self.rmbd_path.display()))?;

        *guard = Some(child);
        Ok(())
    }

    pub fn stop_managed(&self) {
        let mut guard = self.child.lock().unwrap();
        if let Some(mut child) = guard.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }
}

pub fn base_url() -> String {
    std::env::var("RMB_ADDR")
        .map(|v| {
            let v = v.trim().to_string();
            if v.starts_with("http://") || v.starts_with("https://") {
                v
            } else {
                format!("http://{v}")
            }
        })
        .unwrap_or_else(|_| DEFAULT_BASE_URL.to_string())
}

pub fn dashboard_url() -> String {
    format!("{}/ui/", base_url().trim_end_matches('/'))
}

pub fn health_ok(base: &str) -> bool {
    let url = format!("{}/healthz", base.trim_end_matches('/'));
    ureq::get(&url)
        .timeout(std::time::Duration::from_secs(2))
        .call()
        .map(|r| r.status() == 200)
        .unwrap_or(false)
}

fn find_rmbd_binary() -> PathBuf {
    if let Ok(path) = std::env::var("RMBD_PATH") {
        let p = PathBuf::from(path);
        if p.exists() {
            return p;
        }
    }

    if let Some(path) = bootstrap::installed_daemon_path() {
        if path.exists() {
            return path;
        }
    }

    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            for candidate in [
                dir.join("rmbd"),
                dir.join("rmbd-desktop"),
                dir.join("../rmbd"),
                dir.join("../../../bin/rmbd"),
                dir.join("../../../../bin/rmbd"),
            ] {
                if candidate.exists() {
                    return candidate.canonicalize().unwrap_or(candidate);
                }
            }
        }
    }

    if let Ok(path) = which_rmbd() {
        return path;
    }

    PathBuf::from("rmbd")
}

fn which_rmbd() -> Result<PathBuf, ()> {
    let output = Command::new("which").arg("rmbd").output().map_err(|_| ())?;
    if !output.status.success() {
        return Err(());
    }
    let path = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if path.is_empty() {
        return Err(());
    }
    Ok(PathBuf::from(path))
}
