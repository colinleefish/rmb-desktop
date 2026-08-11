use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::Mutex;
use std::thread;
use std::time::Duration;

use crate::bootstrap;

const DEFAULT_BASE_URL: &str = "http://127.0.0.1:19019";
const LAUNCHD_LABEL: &str = "me.remember.rmbd";

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
            .args(config_serve_args())
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

    /// Starts the managed daemon after stopping any launchd-owned copy.
    pub fn ensure_running(&self) -> Result<(), String> {
        if self.managed_running() {
            return Ok(());
        }
        // Dev / launchd may already have a healthy listener — do not kill it.
        if health_ok(DEFAULT_BASE_URL) {
            return Ok(());
        }

        detach_external_daemon();
        wait_for_health(false, Duration::from_secs(3));
        if health_ok(DEFAULT_BASE_URL) {
            return Ok(());
        }

        self.start()?;
        wait_for_health(true, Duration::from_secs(15));
        Ok(())
    }

    /// Stops the managed daemon, unloads launchd, and kills listeners on 19019.
    pub fn shutdown(&self) {
        self.stop_managed();
        detach_external_daemon();
        wait_for_health(false, Duration::from_secs(3));
    }
}

fn wait_for_health(want_healthy: bool, timeout: Duration) {
    let deadline = std::time::Instant::now() + timeout;
    while std::time::Instant::now() < deadline {
        if health_ok(DEFAULT_BASE_URL) == want_healthy {
            return;
        }
        thread::sleep(Duration::from_millis(200));
    }
}

#[cfg(target_os = "macos")]
fn detach_external_daemon() {
    let Some(uid) = gui_uid() else {
        kill_listeners_on_port(19019);
        return;
    };
    let domain = format!("gui/{uid}");

    let _ = Command::new("launchctl")
        .args(["disable", &domain, LAUNCHD_LABEL])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();

    if let Some(plist) = launchd_plist_path() {
        let _ = Command::new("launchctl")
            .args(["bootout", &domain, plist.to_string_lossy().as_ref()])
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
    }

    let _ = Command::new("launchctl")
        .args(["bootout", &domain, LAUNCHD_LABEL])
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();

    kill_listeners_on_port(19019);
}

#[cfg(not(target_os = "macos"))]
fn detach_external_daemon() {
    kill_listeners_on_port(19019);
}

#[cfg(target_os = "macos")]
fn launchd_plist_path() -> Option<PathBuf> {
    dirs::home_dir().map(|home| {
        home.join("Library")
            .join("LaunchAgents")
            .join(format!("{LAUNCHD_LABEL}.plist"))
    })
}

fn gui_uid() -> Option<String> {
    let output = Command::new("id").arg("-u").output().ok()?;
    if !output.status.success() {
        return None;
    }
    let uid = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if uid.is_empty() {
        None
    } else {
        Some(uid)
    }
}

fn kill_listeners_on_port(port: u16) {
    let Ok(output) = Command::new("lsof")
        .args(["-ti", &format!(":{port}")])
        .output()
    else {
        return;
    };
    if !output.status.success() {
        return;
    }
    for line in String::from_utf8_lossy(&output.stdout).lines() {
        let pid = line.trim();
        if pid.is_empty() {
            continue;
        }
        let _ = Command::new("kill")
            .arg(pid)
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .status();
    }
    thread::sleep(Duration::from_millis(300));
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

fn config_serve_args() -> Vec<String> {
    let Some(home) = dirs::home_dir() else {
        return Vec::new();
    };
    let config = home
        .join("Library")
        .join("Application Support")
        .join("rmb-desktop")
        .join("config.yaml");
    if config.is_file() {
        vec![
            "-config".to_string(),
            config.to_string_lossy().into_owned(),
        ]
    } else {
        Vec::new()
    }
}
