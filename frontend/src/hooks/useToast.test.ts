/**
 * Toast 文案提取：保证页面捕获到任意异常时都有可读提示。
 */
import { describe, expect, it } from 'vitest'
import { toErrorMessage } from './useToast'

describe('toErrorMessage', () => {
  it('keeps the message of an Error', () => {
    expect(toErrorMessage(new Error('网络连接失败'))).toBe('网络连接失败')
  })

  it('accepts plain strings from legacy rejections', () => {
    expect(toErrorMessage('请求超时')).toBe('请求超时')
  })

  it('reads message-like objects', () => {
    expect(toErrorMessage({ message: '权限不足' })).toBe('权限不足')
  })

  it('falls back when the error has no usable text', () => {
    expect(toErrorMessage(null, '导出失败')).toBe('导出失败')
    expect(toErrorMessage(new Error(''), '导出失败')).toBe('导出失败')
    expect(toErrorMessage(500, '导出失败')).toBe('导出失败')
  })
})
