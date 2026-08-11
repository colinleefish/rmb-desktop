use std::fs::OpenOptions;
use std::io::Write;
use std::os::unix::io::AsRawFd;
use std::path::PathBuf;

pub struct InstanceLock {
    _file: std::fs::File,
}

pub fn acquire() -> Result<InstanceLock, String> {
    let path = lock_path()?;
    std::fs::create_dir_all(
        path.parent()
            .ok_or_else(|| "invalid lock path".to_string())?,
    )
    .map_err(|e| format!("create lock dir: {e}"))?;

    let mut file = OpenOptions::new()
        .create(true)
        .write(true)
        .open(&path)
        .map_err(|e| format!("open lock file: {e}"))?;

    let ret = unsafe { libc::flock(file.as_raw_fd(), libc::LOCK_EX | libc::LOCK_NB) };
    if ret != 0 {
        return Err("RMB is already running".into());
    }

    file.set_len(0)
        .map_err(|e| format!("truncate lock file: {e}"))?;
    writeln!(file, "{}", std::process::id()).map_err(|e| format!("write lock file: {e}"))?;

    Ok(InstanceLock { _file: file })
}

fn lock_path() -> Result<PathBuf, String> {
    dirs::home_dir()
        .map(|home| home.join(".rmb").join("rmb-app.lock"))
        .ok_or_else(|| "home directory not found".to_string())
}
