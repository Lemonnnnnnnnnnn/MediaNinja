fn main() {
    println!(
        "cargo:rustc-env=MEDIANINJA_TARGET={}",
        std::env::var("TARGET").unwrap()
    );
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            "select_output_directory",
            "open_output_directory",
            "notify_task",
        ]),
    ))
    .expect("Unable to build desktop permissions");
}
