import { describe, expect, test } from 'bun:test'
import { Handler } from '../../server/routes/translation-assets/yon.js'

describe('translation asset route', () => {
  test('accepts only bounded model and runtime paths', () => {
    expect(Handler.readAssetPath('models/casawolice/small100-onnx/config.json')).toBe('models/casawolice/small100-onnx/config.json')
    expect(Handler.readAssetPath('runtime/ort-wasm-simd-threaded.asyncify.wasm')).toBe('runtime/ort-wasm-simd-threaded.asyncify.wasm')
    expect(Handler.readAssetPath('../runtime/secret')).toBeInstanceOf(Error)
    expect(Handler.readAssetPath('/etc/passwd')).toBeInstanceOf(Error)
    expect(Handler.readAssetPath('other/file.bin')).toBeInstanceOf(Error)
  })

  test('returns explicit safe content types', () => {
    expect(Handler.contentType('model.json')).toBe('application/json; charset=utf-8')
    expect(Handler.contentType('runtime.wasm')).toBe('application/wasm')
    expect(Handler.contentType('part.bin')).toBe('application/octet-stream')
  })

})
