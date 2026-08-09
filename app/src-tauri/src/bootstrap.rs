use std::fs;
use std::io;
use std::path::{Path, PathBuf};

const INSTALL_DIR: &str = ".rmb/bin";
const CLI_NAME: &str = "rmb";
const DAEMON_NAME: &str = "rmbd-desktop";

pub fn ensure_installed() -> Result<(), String> {
    let home = dirs::home_dir().ok_or("could not resolve home directory")?;
    let install_dir = home.join(INSTALL_DIR);
    fs::create_dir_all(&install_dir).map_err(|e| format!("create {}: {e}", install_dir.display()))?;

    let cli_dst = install_dir.join(CLI_NAME);
    let daemon_dst = install_dir.join(DAEMON_NAME);

    if needs_refresh(&cli_dst, &daemon_dst)? {
        install_sidecar("rmb", &cli_dst)?;
        install_sidecar("rmbd", &daemon_dst)?;
    }

    Ok(())
}

pub fn installed_daemon_path() -> Option<PathBuf> {
    dirs::home_dir().map(|home| home.join(INSTALL_DIR).join(DAEMON_NAME))
}

fn needs_refresh(cli_dst: &Path, daemon_dst: &Path) -> Result<bool, String> {
    if !cli_dst.exists() || !daemon_dst.exists() {
        return Ok(true);
    }

    let bundled_rmb = bundled_sidecar_path("rmb")?;
    let bundled_rmbd = bundled_sidecar_path("rmbd")?;

    Ok(is_newer(&bundled_rmb, cli_dst)? || is_newer(&bundled_rmbd, daemon_dst)?)
}

fn is_newer(src: &Path, dst: &Path) -> Result<bool, String> {
    let src_meta = fs::metadata(src).map_err(|e| format!("stat {}: {e}", src.display()))?;
    let dst_meta = fs::metadata(dst).map_err(|e| format!("stat {}: {e}", dst.display()))?;
    let src_mtime = src_meta.modified().map_err(io_err)?;
    let dst_mtime = dst_meta.modified().map_err(io_err)?;
    Ok(src_mtime > dst_mtime)
}

fn install_sidecar(base_name: &str, dst: &Path) -> Result<(), String> {
    let src = bundled_sidecar_path(base_name)?;
    if let Some(parent) = dst.parent() {
        fs::create_dir_all(parent).map_err(|e| format!("create {}: {e}", parent.display()))?;
    }
    fs::copy(&src, dst).map_err(|e| format!("copy {} -> {}: {e}", src.display(), dst.display()))?;
    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        let mut perms = fs::metadata(dst)
            .map_err(|e| format!("stat {}: {e}", dst.display()))?
            .permissions();
        perms.set_mode(0o755);
        fs::set_permissions(dst, perms).map_err(|e| format!("chmod {}: {e}", dst.display()))?;
    }
    Ok(())
}

fn bundled_sidecar_path(base_name: &str) -> Result<PathBuf, String> {
    let exe = std::env::current_exe().map_err(|e| format!("current_exe: {e}"))?;
    let exe_dir = exe
        .parent()
        .ok_or_else(|| "current_exe has no parent".to_string())?;

    let candidates = [
        exe_dir.join(base_name),
        #[cfg(windows)]
        exe_dir.join(format!("{base_name}.exe")),
    ];

    for candidate in candidates {
        if candidate.is_file() {
            return Ok(candidate);
        }
    }

    if let Some(found) = find_prefixed_binary(exe_dir, base_name) {
        return Ok(found);
    }

    // Dev fallback: repo bin/ when running from target/debug or target/release.
    if let Some(dir) = exe.parent() {
        for dev_bin in [
            dir.join("../../../bin").join(base_name),
            dir.join("../../../../bin").join(base_name),
        ] {
            if dev_bin.is_file() {
                return dev_bin
                    .canonicalize()
                    .map_err(|e| format!("canonicalize {}: {e}", dev_bin.display()));
            }
        }
    }

    Err(format!(
        "bundled sidecar {base_name} not found near {}",
        exe.display()
    ))
}

fn find_prefixed_binary(dir: &Path, base_name: &str) -> Option<PathBuf> {
    let entries = fs::read_dir(dir).ok()?;
    let prefix = format!("{base_name}-");
    for entry in entries.flatten() {
        let path = entry.path();
        if !path.is_file() {
            continue;
        }
        let name = entry.file_name();
        let name = name.to_string_lossy();
        if name == base_name || name.starts_with(&prefix) {
            return Some(path);
        }
    }
    None
}

fn io_err(err: io::Error) -> String {
    err.to_string()
}
