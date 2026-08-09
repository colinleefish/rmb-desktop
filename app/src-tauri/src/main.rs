// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod bootstrap;
mod daemon;

use daemon::{DaemonManager, dashboard_url, health_ok};
use std::sync::Arc;
use std::thread;
use std::time::Duration;
use tauri::{
    AppHandle, CustomMenuItem, RunEvent, SystemTray, SystemTrayEvent, SystemTrayMenu,
    SystemTrayMenuItem,
};

const ID_STATUS: &str = "status";
const ID_OPEN: &str = "open_dashboard";
const ID_START: &str = "start_rmbd";
const ID_STOP: &str = "stop_rmbd";
const ID_QUIT: &str = "quit";

fn main() {
    let status = CustomMenuItem::new(ID_STATUS, "○ Checking…").disabled();
    let open = CustomMenuItem::new(ID_OPEN, "Open Dashboard");
    let start = CustomMenuItem::new(ID_START, "Start rmbd");
    let stop = CustomMenuItem::new(ID_STOP, "Stop rmbd").disabled();
    let quit = CustomMenuItem::new(ID_QUIT, "Quit");

    let tray_menu = SystemTrayMenu::new()
        .add_item(status)
        .add_native_item(SystemTrayMenuItem::Separator)
        .add_item(open)
        .add_item(start)
        .add_item(stop)
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
            move |_app| {
                if let Err(err) = bootstrap::ensure_installed() {
                    eprintln!("bootstrap: {err}");
                }
                let handle = _app.handle();
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
            daemon_for_exit.stop_managed();
        }
    });
}

fn on_tray_event(app: &AppHandle, event: SystemTrayEvent, daemon: &Arc<DaemonManager>) {
    if let SystemTrayEvent::MenuItemClick { id, .. } = event {
        match id.as_str() {
            ID_OPEN => {
                let url = dashboard_url();
                if let Err(err) = open::that(&url) {
                    eprintln!("open dashboard: {err}");
                }
            }
            ID_START => {
                if let Err(err) = daemon.start() {
                    eprintln!("start rmbd: {err}");
                }
                refresh_menu(app, daemon);
            }
            ID_STOP => {
                daemon.stop_managed();
                refresh_menu(app, daemon);
            }
            ID_QUIT => {
                daemon.stop_managed();
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
    let managed = daemon.managed_running();

    let status_label = if healthy {
        "● rmbd running"
    } else if managed {
        "◐ rmbd starting…"
    } else {
        "○ rmbd stopped"
    };
    let _ = tray.get_item(ID_STATUS).set_title(status_label);

    let _ = tray.get_item(ID_START).set_enabled(!healthy && !managed);
    let _ = tray.get_item(ID_STOP).set_enabled(managed);
    let _ = tray.get_item(ID_OPEN).set_enabled(healthy);
}
