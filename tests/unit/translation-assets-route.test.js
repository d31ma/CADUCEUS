import { describe, expect, test } from 'bun:test'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

const Handler = Routes.TranslationAssetsController

describe('translation asset route', () => {
  test('accepts only bounded model and runtime paths', () => {
    expect(Handler._readAssetPath('models/casawolice/small100-onnx/config.json')).toBe('models/casawolice/small100-onnx/config.json')
    expect(Handler._readAssetPath('runtime/ort-wasm-simd-threaded.asyncify.wasm')).toBe('runtime/ort-wasm-simd-threaded.asyncify.wasm')
    expect(Handler._readAssetPath('../runtime/secret')).toBeInstanceOf(Error)
    expect(Handler._readAssetPath('/etc/passwd')).toBeInstanceOf(Error)
    expect(Handler._readAssetPath('other/file.bin')).toBeInstanceOf(Error)
  })

  test('returns explicit safe content types', () => {
    expect(Handler._contentType('model.json')).toBe('application/json; charset=utf-8')
    expect(Handler._contentType('runtime.wasm')).toBe('application/wasm')
    expect(Handler._contentType('part.bin')).toBe('application/octet-stream')
  })

})
