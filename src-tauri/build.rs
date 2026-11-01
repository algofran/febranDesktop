use std::fs;

fn main() {
    // The central API key is shared by every installed node, so it stays out of the source tree:
    // take it from the build environment or from an untracked file next to this script.
    println!("cargo:rerun-if-env-changed=FEBRAN_CENTRAL_API_KEY");
    println!("cargo:rerun-if-changed=central-key.txt");
    let key = std::env::var("FEBRAN_CENTRAL_API_KEY")
        .ok()
        .or_else(|| fs::read_to_string("central-key.txt").ok())
        .map(|k| k.trim().to_string())
        .filter(|k| !k.is_empty());
    match key {
        Some(k) => println!("cargo:rustc-env=FEBRAN_CENTRAL_API_KEY={k}"),
        None => println!("cargo:warning=FEBRAN_CENTRAL_API_KEY / src-tauri/central-key.txt not set: building with the development central key"),
    }
    tauri_build::build()
}
