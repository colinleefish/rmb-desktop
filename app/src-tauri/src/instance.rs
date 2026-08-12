use std::path::PathBuf;

pub struct InstanceLock {
    #[cfg(unix)]
    _file: std::fs::File,
    #[cfg(windows)]
    _handle: windows_sys::Win32::Foundation::HANDLE,
}

pub fn acquire() -> Result<InstanceLock, String> {
    #[cfg(unix)]
    {
        return acquire_unix();
    }
    #[cfg(windows)]
    {
        return acquire_windows();
    }
}

#[cfg(unix)]
fn acquire_unix() -> Result<InstanceLock, String> {
    use std::fs::OpenOptions;
    use std::io::Write;
    use std::os::unix::io::AsRawFd;

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

#[cfg(windows)]
fn acquire_windows() -> Result<InstanceLock, String> {
    use std::ffi::OsStr;
    use std::os::windows::ffi::OsStrExt;
    use std::ptr::null_mut;
    use windows_sys::Win32::Foundation::{CloseHandle, GetLastError, WAIT_OBJECT_0, WAIT_TIMEOUT};
    use windows_sys::Win32::System::Threading::{CreateMutexW, WaitForSingleObject};

    let name: Vec<u16> = OsStr::new("Global\\me.remember.rmb.app")
        .encode_wide()
        .chain(std::iter::once(0))
        .collect();

    let handle = unsafe { CreateMutexW(null_mut(), 0, name.as_ptr()) };
    if handle == 0 {
        return Err(format!("CreateMutex failed: {}", unsafe { GetLastError() }));
    }

    let wait = unsafe { WaitForSingleObject(handle, 0) };
    if wait == WAIT_TIMEOUT {
        unsafe {
            CloseHandle(handle);
        }
        return Err("RMB is already running".into());
    }
    if wait != WAIT_OBJECT_0 {
        unsafe {
            CloseHandle(handle);
        }
        return Err(format!("WaitForSingleObject failed: {wait}"));
    }

    Ok(InstanceLock { _handle: handle })
}

#[cfg(windows)]
impl Drop for InstanceLock {
    fn drop(&mut self) {
        use windows_sys::Win32::Foundation::CloseHandle;
        use windows_sys::Win32::System::Threading::ReleaseMutex;

        unsafe {
            ReleaseMutex(self._handle);
            CloseHandle(self._handle);
        }
    }
}

fn lock_path() -> Result<PathBuf, String> {
    dirs::home_dir()
        .map(|home| home.join(".rmb").join("rmb-app.lock"))
        .ok_or_else(|| "home directory not found".to_string())
}
