// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod bootstrap;
mod daemon;
mod instance;

use daemon::{DaemonManager, dashboard_url, health_ok};
use std::sync::Arc;
use std::thread;
use std::time::Duration;
use tauri::{
    AppHandle, CustomMenuItem, RunEvent, SystemTray, SystemTrayEvent, SystemTrayMenu,
    SystemTrayMenuItem,
};

const ID_STATUS: &str = "status";
const ID_QUIT: &str = "quit";

fn main() {
    let _instance_lock = match instance::acquire() {
        Ok(lock) => lock,
        Err(_) => return,
    };

    let status = CustomMenuItem::new(ID_STATUS, "Starting…").disabled();
    let quit = CustomMenuItem::new(ID_QUIT, "Quit RMB");

    let tray_menu = SystemTrayMenu::new()
        .add_item(status)
        .add_native_item(SystemTrayMenuItem::Separator)
        .add_item(quit);

    let tray = SystemTray::new().with_menu(tray_menu);
    let daemon = Arc::new(DaemonManager::new());

    let mut app = tauri::Builder::default()
        .system_tray(tray)
        .on_system_tray_event({
            let daemon = Arc::clone(&daemon);
            move |app, event| on_tray_event(app, event, &daemon)
        })
        .setup({
            let daemon = Arc::clone(&daemon);
            move |app| {
                if let Err(err) = bootstrap::ensure_installed() {
                    eprintln!("bootstrap: {err}");
                }
                if let Err(err) = daemon.ensure_running() {
                    eprintln!("start rmbd: {err}");
                }
                let handle = app.handle();
                refresh_menu(&handle, &daemon);
                spawn_health_poller(handle, daemon);
                Ok(())
            }
        })
        .build(tauri::generate_context!())
        .expect("error building tauri application");

    #[cfg(target_os = "macos")]
    app.set_activation_policy(tauri::ActivationPolicy::Accessory);

    let daemon_for_exit = Arc::clone(&daemon);
    app.run(move |_, event| {
        if let RunEvent::ExitRequested { .. } = event {
            daemon_for_exit.shutdown();
        }
    });
}

fn on_tray_event(app: &AppHandle, event: SystemTrayEvent, daemon: &Arc<DaemonManager>) {
    if let SystemTrayEvent::MenuItemClick { id, .. } = event {
        match id.as_str() {
            ID_STATUS => {
                if !health_ok(&daemon::base_url()) {
                    return;
                }
                let url = dashboard_url();
                if let Err(err) = open::that(&url) {
                    eprintln!("open dashboard: {err}");
                }
            }
            ID_QUIT => {
                daemon.shutdown();
                app.exit(0);
            }
            _ => {}
        }
    }
}

fn spawn_health_poller(app: AppHandle, daemon: Arc<DaemonManager>) {
    thread::spawn(move || loop {
        refresh_menu(&app, &daemon);
        thread::sleep(Duration::from_secs(5));
    });
}

fn refresh_menu(app: &AppHandle, daemon: &DaemonManager) {
    let tray = app.tray_handle();
    let healthy = health_ok(&daemon::base_url());

    let status_label = if healthy {
        "🟢 RMB is running"
    } else {
        "Starting…"
    };
    let _ = tray.get_item(ID_STATUS).set_title(status_label);
    let _ = tray.get_item(ID_STATUS).set_enabled(healthy);
}
