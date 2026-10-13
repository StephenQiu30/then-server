# Local garment background removal

- `u2netp.onnx`: compact U²-Net model from the [rembg official release](https://github.com/danielgatis/rembg/releases/download/v0.0.0/u2netp.onnx), 4,574,861 bytes; upstream MD5 `8e83ca70e441ab06c318d82300c84806`. Original project: [U-2-Net](https://github.com/xuebinqin/U-2-Net), Apache-2.0; license retained in `U2NET-LICENSE.txt`.
- `onnxruntime-web` is pinned in package-lock.json (currently 1.30.0), MIT; official license retained in `ORT-LICENSE.txt`. Its WASM/MJS assets are copied from node_modules before dev/build and are not separately committed.
- The browser loads model/runtime assets from this origin and runs CPU WASM inference locally. No photograph is sent to a third-party inference service. The segmentation mask changes alpha only; RGB comes from the input image. Original files and processed PNGs are stored separately through the existing authenticated media flow.
- This is a small general foreground model. Users review edges, may crop first, and can restore the original. It does not guarantee successful segmentation of every fabric/background.
