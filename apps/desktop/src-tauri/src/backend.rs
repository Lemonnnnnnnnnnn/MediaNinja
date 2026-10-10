use serde::Deserialize;
#[cfg(windows)]
use std::os::windows::process::CommandExt;
use std::{
    io::{BufRead, BufReader},
    path::PathBuf,
    process::{Child, Command, Stdio},
    sync::{
        atomic::{AtomicBool, Ordering},
        mpsc, Mutex,
    },
    time::{Duration, Instant},
};

#[derive(Deserialize)]
struct Ready {
    protocol: u32,
    url: String,
}
pub struct Backend {
    child: Mutex<Option<Child>>,
    stopping: AtomicBool,
    pub origin: String,
}
impl Backend {
    pub fn start(output: &PathBuf) -> Result<Self, String> {
        let exe = std::env::current_exe().map_err(|e| e.to_string())?;
        let mut binary = exe
            .parent()
            .ok_or("Missing executable directory")?
            .join("medianinja-core.exe");
        if cfg!(debug_assertions) && !binary.exists() {
            binary = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join(format!(
                "binaries/medianinja-core-{}.exe",
                env!("MEDIANINJA_TARGET")
            ));
        }
        let mut command = Command::new(binary);
        command
            .args(["serve", "--desktop", "--listen", "127.0.0.1:0", "--output"])
            .arg(output)
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit());
        if cfg!(debug_assertions) {
            command
                .arg("--frontend-dir")
                .arg(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../frontend/dist"));
        }
        #[cfg(windows)]
        command.creation_flags(0x08000000); // CREATE_NO_WINDOW: the sidecar has no terminal.
        let mut child = command
            .spawn()
            .map_err(|e| format!("Unable to launch Go service: {e}"))?;
        let stdout = child.stdout.take().ok_or("Missing backend stdout")?;
        let (tx, rx) = mpsc::channel();
        std::thread::spawn(move || {
            let mut reader = BufReader::new(stdout);
            let mut line = String::new();
            let result = reader.read_line(&mut line).map(|_| line);
            let _ = tx.send(result);
            // Drain stdout while the backend runs, preventing a full pipe from blocking it.
            for _ in reader.lines() {}
        });
        let ready = match rx.recv_timeout(Duration::from_secs(20)) {
            Ok(Ok(line)) => serde_json::from_str::<Ready>(&line).map_err(|e| e.to_string()),
            _ => Err("Go service did not become ready within 20 seconds".into()),
        };
        let info = match ready {
            Ok(info) if info.protocol == 1 && info.url.starts_with("http://127.0.0.1:") => info,
            Ok(_) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err("Unsupported backend protocol".into());
            }
            Err(error) => {
                let _ = child.kill();
                let _ = child.wait();
                return Err(error);
            }
        };
        Ok(Self {
            child: Mutex::new(Some(child)),
            stopping: AtomicBool::new(false),
            origin: info.url,
        })
    }
    pub fn stop(&self) {
        self.stopping.store(true, Ordering::Release);
        let Ok(mut guard) = self.child.lock() else {
            return;
        };
        let Some(mut child) = guard.take() else {
            return;
        };
        drop(child.stdin.take()); // EOF requests graceful cancellation and shutdown in Go.
        let deadline = Instant::now() + Duration::from_secs(8);
        loop {
            match child.try_wait() {
                Ok(Some(_)) => return,
                Err(_) => break,
                Ok(None) if Instant::now() < deadline => {
                    std::thread::sleep(Duration::from_millis(50))
                }
                _ => break,
            }
        }
        let _ = child.kill();
        let _ = child.wait();
    }
    pub fn exited(&self) -> bool {
        if self.stopping.load(Ordering::Acquire) {
            return false;
        }
        self.child
            .lock()
            .ok()
            .and_then(|mut guard| {
                guard
                    .as_mut()
                    .map(|child| child.try_wait().ok().flatten().is_some())
            })
            .unwrap_or(true)
    }
}
impl Drop for Backend {
    fn drop(&mut self) {
        self.stop();
    }
}
