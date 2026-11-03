import { describe, expect, it } from 'vitest'
import { base64urlToBuffer, bufferToBase64url, passkeyErrorMessage } from './passkey'

describe('passkey base64url helpers', () => {
  it('round-trips arbitrary bytes without padding', () => {
    const bytes = new Uint8Array([0, 1, 2, 127, 128, 250, 251, 252, 253, 254, 255])
    const encoded = bufferToBase64url(bytes.buffer)
    expect(encoded).not.toContain('=')
    expect(encoded).not.toContain('+')
    expect(encoded).not.toContain('/')
    expect(new Uint8Array(base64urlToBuffer(encoded))).toEqual(bytes)
  })

  it('decodes url-safe input and tolerates missing padding', () => {
    const encoded = bufferToBase64url(new Uint8Array([251, 255]).buffer)
    expect(encoded).toBe('-_8')
    expect(new Uint8Array(base64urlToBuffer('-w'))).toEqual(new Uint8Array([251]))
    expect(new Uint8Array(base64urlToBuffer(encoded))).toEqual(new Uint8Array([251, 255]))
  })

  it('turns browser cancellation into a readable message', () => {
    expect(passkeyErrorMessage(new DOMException('cancel', 'NotAllowedError'))).toContain('取消')
    expect(passkeyErrorMessage(new Error('网络错误'))).toBe('网络错误')
  })
})
