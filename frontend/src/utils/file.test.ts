/**
 * 文件类型识别与筛选工具测试。
 */
import { describe, expect, it } from 'vitest'
import {
  acceptForKinds,
  extensionOf,
  fileKind,
  fileKindQuery,
  fileMatchesKinds,
  fileNameFromURL,
  normalizeFileKinds,
} from './file'

describe('file kinds', () => {
  it('classifies by mime and extension', () => {
    expect(fileKind({ mime: 'image/png', ext: 'png' })).toBe('image')
    expect(fileKind({ mime: 'application/octet-stream', ext: 'MP4' })).toBe('video')
    expect(fileKind({ mime: 'audio/mpeg', ext: 'mp3' })).toBe('audio')
    expect(fileKind({ mime: 'application/zip', ext: 'docx' })).toBe('document')
    expect(fileKind({ mime: 'application/zip', ext: 'zip' })).toBe('archive')
    expect(fileKind({ name: 'notes.md', mime: 'text/markdown' })).toBe('document')
    expect(fileKind({ name: 'blob.bin', mime: 'application/octet-stream' })).toBe('other')
  })

  it('normalizes and serializes allowed kinds', () => {
    expect(normalizeFileKinds(['video', 'image', 'video'])).toEqual(['image', 'video'])
    expect(fileKindQuery(['video', 'image'])).toBe('image,video')
    expect(fileKindQuery(['image', 'video', 'audio', 'document', 'archive', 'other'])).toBeUndefined()
    expect(fileKindQuery()).toBeUndefined()
  })

  it('builds accept filters and checks membership', () => {
    expect(acceptForKinds(['image', 'video'])).toBe('image/*,video/*')
    expect(acceptForKinds(['document'])).toContain('.pdf')
    expect(acceptForKinds(['other'])).toBe('')
    expect(fileMatchesKinds({ ext: 'png' }, ['image'])).toBe(true)
    expect(fileMatchesKinds({ ext: 'png' }, ['video'])).toBe(false)
    expect(fileMatchesKinds({ ext: 'png' }, undefined)).toBe(true)
  })

  it('extracts names and extensions from urls', () => {
    expect(fileNameFromURL('/uploads/2026/11/abc.png?x=1')).toBe('abc.png')
    expect(extensionOf('https://cdn.example.com/a/b/cover.JPG?w=200')).toBe('jpg')
    expect(extensionOf('README')).toBe('')
  })
})
