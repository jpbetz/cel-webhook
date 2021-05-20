use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub fn validate_length(x: String) -> bool {
  return x.len() < 10
}
