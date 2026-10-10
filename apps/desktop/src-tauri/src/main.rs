#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]
mod backend;
use backend::Backend;
use std::path::PathBuf;
use tauri::menu::{Menu, MenuItem};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{Manager, State, WebviewUrl, WebviewWindow, WebviewWindowBuilder};
use tauri_plugin_dialog::DialogExt;
use tauri_plugin_notification::NotificationExt;

fn verify(window: &WebviewWindow, backend: &Backend) -> Result<(), String> {
    let url = window.url().map_err(|e| e.to_string())?;
    if window.label() != "main" || url.origin().ascii_serialization() != backend.origin {
        return Err("Desktop commands are restricted to the MediaNinja application".into());
    }
    Ok(())
}
#[tauri::command]
async fn select_output_directory(
    app: tauri::AppHandle,
    window: WebviewWindow,
    backend: State<'_, Backend>,
) -> Result<Option<String>, String> {
    verify(&window, &backend)?;
    let path = app
        .dialog()
        .file()
        .set_title("选择 MediaNinja 下载目录")
        .blocking_pick_folder();
    path.map(|path| {
        path.into_path()
            .map(|path| path.to_string_lossy().into_owned())
            .map_err(|e| e.to_string())
    })
    .transpose()
}
#[tauri::command]
fn open_output_directory(
    window: WebviewWindow,
    backend: State<'_, Backend>,
    path: String,
) -> Result<(), String> {
    verify(&window, &backend)?;
    let directory = PathBuf::from(path);
    if !directory.is_absolute() || !directory.is_dir() {
        return Err("输出目录不存在".into());
    }
    std::process::Command::new("explorer.exe")
        .arg(directory)
        .spawn()
        .map_err(|e| e.to_string())?;
    Ok(())
}
#[tauri::command]
fn notify_task(
    app: tauri::AppHandle,
    window: WebviewWindow,
    backend: State<'_, Backend>,
    title: String,
    message: String,
) -> Result<(), String> {
    verify(&window, &backend)?;
    if title.len() > 128 || message.len() > 1024 {
        return Err("Notification is too long".into());
    }
    app.notification()
        .builder()
        .title(title)
        .body(message)
        .show()
        .map_err(|e| e.to_string())
}
fn show_main(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.unminimize();
        let _ = window.show();
        let _ = window.set_focus();
    }
}
fn quit(app: &tauri::AppHandle) {
    let app = app.clone();
    std::thread::spawn(move || {
        app.state::<Backend>().stop();
        app.exit(0);
    });
}
fn main() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_notification::init())
        .invoke_handler(tauri::generate_handler![
            select_output_directory,
            open_output_directory,
            notify_task
        ])
        .setup(|app| {
            let output = if cfg!(debug_assertions) {
                PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../downloads")
            } else {
                app.path().download_dir()?.join("MediaNinja")
            };
            let backend = Backend::start(&output).map_err(std::io::Error::other)?;
            let origin: tauri::Url = backend.origin.parse()?;
            let allowed_origin = backend.origin.clone();
            app.manage(backend);
            let window = WebviewWindowBuilder::new(app, "main", WebviewUrl::External(origin))
                .title("MediaNinja")
                .inner_size(1180.0, 800.0)
                .min_inner_size(800.0, 600.0)
                .on_navigation(move |url| url.origin().ascii_serialization() == allowed_origin)
                .build()?;
            let hide_window = window.clone();
            window.on_window_event(move |event| {
                if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                    api.prevent_close();
                    let _ = hide_window.hide();
                }
            });
            let open = MenuItem::with_id(app, "open", "打开 MediaNinja", true, None::<&str>)?;
            let exit = MenuItem::with_id(app, "quit", "退出并停止任务", true, None::<&str>)?;
            let menu = Menu::with_items(app, &[&open, &exit])?;
            let icon = tauri::image::Image::from_bytes(include_bytes!("../icons/icon.png"))?;
            TrayIconBuilder::new()
                .icon(icon)
                .tooltip("MediaNinja · 后台下载")
                .menu(&menu)
                .show_menu_on_left_click(false)
                .on_menu_event(|app, event| match event.id.as_ref() {
                    "open" => show_main(app),
                    "quit" => quit(app),
                    _ => {}
                })
                .on_tray_icon_event(|tray, event| {
                    if matches!(
                        event,
                        TrayIconEvent::Click {
                            button: MouseButton::Left,
                            button_state: MouseButtonState::Up,
                            ..
                        }
                    ) {
                        show_main(tray.app_handle());
                    }
                })
                .build(app)?;
            let handle = app.handle().clone();
            std::thread::spawn(move || loop {
                std::thread::sleep(std::time::Duration::from_secs(2));
                if handle.state::<Backend>().exited() {
                    show_main(&handle);
                    handle
                        .dialog()
                        .message("下载服务已停止，请退出并重新打开 MediaNinja。")
                        .title("MediaNinja")
                        .show(|_| {});
                    break;
                }
            });
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("Unable to start MediaNinja desktop");
    app.run(|handle, event| {
        if matches!(event, tauri::RunEvent::Exit) {
            handle.state::<Backend>().stop();
        }
    });
}
